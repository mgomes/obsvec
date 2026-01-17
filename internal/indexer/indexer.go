package indexer

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/mgomes/obsvec/internal/cohere"
	"github.com/mgomes/obsvec/internal/db"
)

const (
	maxChunkTokens   = 500
	batchSize        = 96
	avgCharsPerToken = 4
)

type Indexer struct {
	db     *db.DB
	cohere *cohere.Client
	dir    string
}

type Chunk struct {
	Content   string
	StartLine int
	EndLine   int
	Heading   string
}

type pendingChunk struct {
	chunkID int64
	content string
}

type Progress struct {
	Current  int
	Total    int
	FilePath string
	Message  string
}

type ProgressFunc func(Progress)

var headingRegex = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

func New(database *db.DB, cohereClient *cohere.Client, obsidianDir string) *Indexer {
	return &Indexer{
		db:     database,
		cohere: cohereClient,
		dir:    obsidianDir,
	}
}

func (idx *Indexer) Index(ctx context.Context, fullReindex bool, progress ProgressFunc) error {
	files, err := idx.findMarkdownFiles()
	if err != nil {
		return fmt.Errorf("failed to find markdown files: %w", err)
	}

	existingDocs, err := idx.db.GetAllDocuments()
	if err != nil {
		return fmt.Errorf("failed to get existing documents: %w", err)
	}

	existingByPath := make(map[string]*db.Document, len(existingDocs))
	for i := range existingDocs {
		existingByPath[existingDocs[i].Path] = &existingDocs[i]
	}

	currentPaths := make(map[string]bool)
	for _, f := range files {
		currentPaths[f] = true
	}

	for _, doc := range existingDocs {
		if !currentPaths[doc.Path] {
			if progress != nil {
				progress(Progress{Message: fmt.Sprintf("Removing deleted: %s", filepath.Base(doc.Path))})
			}
			if err := idx.db.DeleteDocument(doc.Path); err != nil {
				return fmt.Errorf("failed to delete document %s: %w", doc.Path, err)
			}
		}
	}

	var filesToIndex []string
	for i, filePath := range files {
		if progress != nil {
			progress(Progress{Current: i + 1, Total: len(files), FilePath: filePath, Message: "Checking files..."})
		}

		needsIndex, err := idx.needsIndexing(filePath, fullReindex, existingByPath[filePath])
		if err != nil {
			return err
		}
		if needsIndex {
			filesToIndex = append(filesToIndex, filePath)
		}
	}

	if len(filesToIndex) == 0 {
		if progress != nil {
			progress(Progress{Message: "Index is up to date"})
		}
		return nil
	}

	// Phase 1: Parse all files and collect chunks
	var allPending []pendingChunk
	for i, filePath := range filesToIndex {
		if progress != nil {
			progress(Progress{
				Current:  i + 1,
				Total:    len(filesToIndex),
				FilePath: filePath,
				Message:  fmt.Sprintf("Parsing %s", filepath.Base(filePath)),
			})
		}

		pending, err := idx.parseFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", filePath, err)
		}
		allPending = append(allPending, pending...)
	}

	if len(allPending) == 0 {
		if progress != nil {
			progress(Progress{Message: "No chunks to embed"})
		}
		return nil
	}

	// Phase 2: Batch embed all chunks across files
	return idx.embedPending(ctx, allPending, func(batchNum, totalBatches, batchLen int) {
		if progress != nil {
			progress(Progress{
				Current: batchNum,
				Total:   totalBatches,
				Message: fmt.Sprintf("Embedding batch %d/%d (%d chunks)", batchNum, totalBatches, batchLen),
			})
		}
	})
}

func (idx *Indexer) findMarkdownFiles() ([]string, error) {
	var files []string
	err := filepath.Walk(idx.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if isHiddenDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if isMarkdownFile(info.Name()) {
			relPath, err := filepath.Rel(idx.dir, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}

		return nil
	})

	return files, err
}

func (idx *Indexer) needsIndexing(relPath string, fullReindex bool, doc *db.Document) (bool, error) {
	if fullReindex {
		return true, nil
	}

	if doc == nil {
		return true, nil
	}

	absPath := filepath.Join(idx.dir, relPath)
	info, err := os.Stat(absPath)
	if err != nil {
		return false, err
	}

	return info.ModTime().Unix() > doc.ModifiedAt, nil
}

// parseFile parses a file, stores chunks in DB, and returns pending chunks for embedding
func (idx *Indexer) parseFile(relPath string) ([]pendingChunk, error) {
	absPath := filepath.Join(idx.dir, relPath)
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	title := extractTitle(string(content), relPath)

	docID, err := idx.db.UpsertDocument(relPath, title, info.ModTime().Unix(), time.Now().Unix())
	if err != nil {
		return nil, err
	}

	if err := idx.db.DeleteChunksForDocument(docID); err != nil {
		return nil, err
	}

	chunks := chunkMarkdown(string(content))

	if len(chunks) == 0 {
		return nil, nil
	}

	var pending []pendingChunk
	for _, chunk := range chunks {
		chunkID, err := idx.db.InsertChunk(docID, chunk.Content, chunk.StartLine, chunk.EndLine, chunk.Heading)
		if err != nil {
			return nil, err
		}
		pending = append(pending, pendingChunk{
			chunkID: chunkID,
			content: chunk.Content,
		})
	}

	return pending, nil
}

// indexFile is used by the watcher for single-file indexing
func (idx *Indexer) indexFile(ctx context.Context, relPath string) error {
	pending, err := idx.parseFile(relPath)
	if err != nil {
		return err
	}

	return idx.embedPending(ctx, pending, nil)
}

type batchProgressFunc func(batchNum, totalBatches, batchLen int)

func (idx *Indexer) embedPending(ctx context.Context, pending []pendingChunk, onBatch batchProgressFunc) error {
	if len(pending) == 0 {
		return nil
	}

	totalBatches := (len(pending) + batchSize - 1) / batchSize
	for i := 0; i < len(pending); i += batchSize {
		end := i + batchSize
		if end > len(pending) {
			end = len(pending)
		}
		batch := pending[i:end]
		batchNum := (i / batchSize) + 1

		if onBatch != nil {
			onBatch(batchNum, totalBatches, len(batch))
		}

		texts := make([]string, len(batch))
		for j, p := range batch {
			texts[j] = p.content
		}

		embeddings, err := idx.cohere.EmbedDocuments(ctx, texts)
		if err != nil {
			return fmt.Errorf("failed to generate embeddings for batch %d: %w", batchNum, err)
		}

		for j, p := range batch {
			embBytes, err := sqlite_vec.SerializeFloat32(embeddings[j].Embedding)
			if err != nil {
				return fmt.Errorf("failed to serialize embedding: %w", err)
			}

			if err := idx.db.InsertEmbedding(p.chunkID, embBytes); err != nil {
				return fmt.Errorf("failed to insert embedding: %w", err)
			}
		}
	}

	return nil
}

func extractTitle(content, relPath string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}

	base := filepath.Base(relPath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func chunkMarkdown(content string) []Chunk {
	lines := strings.Split(content, "\n")
	var chunks []Chunk
	var currentChunk strings.Builder
	var currentHeading string
	var headingStack []string
	startLine := 1
	currentLine := 1

	flushChunk := func() {
		text := strings.TrimSpace(currentChunk.String())
		if text != "" && len(text) > 20 {
			chunks = append(chunks, Chunk{
				Content:   text,
				StartLine: startLine,
				EndLine:   currentLine - 1,
				Heading:   currentHeading,
			})
		}
		currentChunk.Reset()
		startLine = currentLine
	}

	for _, line := range lines {
		if match := headingRegex.FindStringSubmatch(line); match != nil {
			flushChunk()

			level := len(match[1])
			headingText := match[2]

			for len(headingStack) >= level {
				headingStack = headingStack[:len(headingStack)-1]
			}
			headingStack = append(headingStack, headingText)

			currentHeading = strings.Join(headingStack, " > ")
			startLine = currentLine
		}

		currentChunk.WriteString(line)
		currentChunk.WriteString("\n")

		if currentChunk.Len() > maxChunkTokens*avgCharsPerToken {
			flushChunk()
		}

		currentLine++
	}

	flushChunk()

	return chunks
}
