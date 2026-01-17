package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounceDelay = 2 * time.Second

type Watcher struct {
	indexer   *Indexer
	watcher   *fsnotify.Watcher
	pending   map[string]time.Time
	mu        sync.Mutex
	stop      chan struct{}
	onMessage func(string)
}

func NewWatcher(indexer *Indexer) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &Watcher{
		indexer: indexer,
		watcher: fsw,
		pending: make(map[string]time.Time),
		stop:    make(chan struct{}),
	}, nil
}

func (w *Watcher) SetMessageHandler(fn func(string)) {
	w.onMessage = fn
}

func (w *Watcher) Start(ctx context.Context) error {
	if err := w.addWatchRecursive(w.indexer.dir); err != nil {
		return err
	}

	go w.processEvents(ctx)
	go w.processPending(ctx)

	w.message(fmt.Sprintf("Watching %s for changes...", w.indexer.dir))

	<-ctx.Done()
	return nil
}

func (w *Watcher) Stop() {
	close(w.stop)
	w.watcher.Close()
}

func (w *Watcher) addWatchRecursive(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return w.watcher.Add(path)
		}

		return nil
	})
}

func (w *Watcher) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.message(fmt.Sprintf("Watch error: %v", err))
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	if !strings.HasSuffix(strings.ToLower(event.Name), ".md") {
		return
	}

	relPath, err := filepath.Rel(w.indexer.dir, event.Name)
	if err != nil {
		return
	}

	if strings.HasPrefix(relPath, ".") {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	switch {
	case event.Op&fsnotify.Write == fsnotify.Write,
		event.Op&fsnotify.Create == fsnotify.Create:
		w.pending[relPath] = time.Now()
		w.message(fmt.Sprintf("Detected change: %s", relPath))

	case event.Op&fsnotify.Remove == fsnotify.Remove,
		event.Op&fsnotify.Rename == fsnotify.Rename:
		delete(w.pending, relPath)
		if err := w.indexer.db.DeleteDocument(relPath); err == nil {
			w.message(fmt.Sprintf("Removed from index: %s", relPath))
		}
	}
}

func (w *Watcher) processPending(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-ticker.C:
			w.indexPendingFiles(ctx)
		}
	}
}

func (w *Watcher) indexPendingFiles(ctx context.Context) {
	w.mu.Lock()
	now := time.Now()
	var toIndex []string
	for path, timestamp := range w.pending {
		if now.Sub(timestamp) >= debounceDelay {
			toIndex = append(toIndex, path)
		}
	}
	for _, path := range toIndex {
		delete(w.pending, path)
	}
	w.mu.Unlock()

	for _, relPath := range toIndex {
		w.message(fmt.Sprintf("Indexing: %s", relPath))
		if err := w.indexer.indexFile(ctx, relPath); err != nil {
			w.message(fmt.Sprintf("Error indexing %s: %v", relPath, err))
		} else {
			w.message(fmt.Sprintf("Indexed: %s", relPath))
		}
	}
}

func (w *Watcher) message(msg string) {
	if w.onMessage != nil {
		w.onMessage(msg)
	} else {
		fmt.Println(msg)
	}
}
