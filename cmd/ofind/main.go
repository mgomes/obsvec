package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mgomes/obsvec/internal/cohere"
	"github.com/mgomes/obsvec/internal/config"
	"github.com/mgomes/obsvec/internal/db"
	"github.com/mgomes/obsvec/internal/indexer"
	"github.com/mgomes/obsvec/internal/search"
	"github.com/mgomes/obsvec/internal/tui"
)

func main() {
	query := flag.String("q", "", "search query")
	doIndex := flag.Bool("index", false, "index the obsidian vault")
	fullReindex := flag.Bool("full", false, "full reindex (use with -index)")
	doWatch := flag.Bool("watch", false, "watch for file changes and auto-index")
	doSetup := flag.Bool("setup", false, "run setup wizard")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if *doSetup || cfg.CohereAPIKey == "" {
		if err := runSetup(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Setup failed: %v\n", err)
			os.Exit(1)
		}
	}

	if cfg.CohereAPIKey == "" || cfg.ObsidianDir == "" {
		fmt.Fprintln(os.Stderr, "Please run setup first: ofind -setup")
		os.Exit(1)
	}

	dbPath, err := config.DBPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get database path: %v\n", err)
		os.Exit(1)
	}

	database, err := db.Open(dbPath, cfg.EmbedDim)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close() //nolint:errcheck

	cohereClient := cohere.NewClient(cfg.CohereAPIKey, cfg.EmbedModel, cfg.RerankModel, cfg.EmbedDim)

	switch {
	case *doIndex:
		if err := runIndex(database, cohereClient, cfg, *fullReindex); err != nil {
			fmt.Fprintf(os.Stderr, "Indexing failed: %v\n", err)
			os.Exit(1)
		}

	case *doWatch:
		if err := runWatch(database, cohereClient, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Watch mode failed: %v\n", err)
			os.Exit(1)
		}

	case *query != "":
		if err := runSearch(database, cohereClient, cfg, *query); err != nil {
			fmt.Fprintf(os.Stderr, "Search failed: %v\n", err)
			os.Exit(1)
		}

	default:
		printUsage()
	}
}

func runSetup(cfg *config.Config) error {
	model := newSetupRunner(cfg)
	program := tea.NewProgram(model)

	finalModel, err := program.Run()
	if err != nil {
		return err
	}

	if runner, ok := finalModel.(setupRunner); ok {
		if runner.apiKey != "" && runner.obsidianDir != "" {
			cfg.CohereAPIKey = runner.apiKey
			cfg.ObsidianDir = runner.obsidianDir
			return cfg.Save()
		}
	}

	return fmt.Errorf("setup cancelled")
}

type setupRunner struct {
	setupModel  tui.SetupModel
	cfg         *config.Config
	apiKey      string
	obsidianDir string
}

func newSetupRunner(cfg *config.Config) setupRunner {
	return setupRunner{
		setupModel: tui.NewSetupModel(),
		cfg:        cfg,
	}
}

func (m setupRunner) Init() tea.Cmd {
	return tea.Batch(m.setupModel.Init(), tea.EnableBracketedPaste)
}

func (m setupRunner) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tui.SetupSubmitMsg:
		ctx := context.Background()
		client := cohere.NewClient(msg.APIKey, m.cfg.EmbedModel, m.cfg.RerankModel, m.cfg.EmbedDim)
		if err := client.ValidateAPIKey(ctx); err != nil {
			newModel, _ := m.setupModel.Update(tui.SetupErrorMsg{Error: "Invalid API key: " + err.Error()})
			if sm, ok := newModel.(tui.SetupModel); ok {
				m.setupModel = sm
			}
			return m, nil
		}

		if _, err := os.Stat(msg.ObsidianDir); os.IsNotExist(err) {
			newModel, _ := m.setupModel.Update(tui.SetupErrorMsg{Error: "Directory does not exist"})
			if sm, ok := newModel.(tui.SetupModel); ok {
				m.setupModel = sm
			}
			return m, nil
		}

		m.apiKey = msg.APIKey
		m.obsidianDir = msg.ObsidianDir
		return m, tea.Quit

	default:
		newModel, cmd := m.setupModel.Update(msg)
		if sm, ok := newModel.(tui.SetupModel); ok {
			m.setupModel = sm
		}
		return m, cmd
	}
}

func (m setupRunner) View() string {
	return m.setupModel.View()
}

func runIndex(database *db.DB, cohereClient *cohere.Client, cfg *config.Config, fullReindex bool) error {
	idx := indexer.New(database, cohereClient, cfg.ObsidianDir)

	progress := func(p indexer.Progress) {
		if p.Total > 0 {
			// Clear line and print progress (truncate long messages)
			msg := p.Message
			if len(msg) > 60 {
				msg = msg[:57] + "..."
			}
			fmt.Printf("\r\033[K[%d/%d] %s", p.Current, p.Total, msg)
		} else if p.Message != "" {
			fmt.Println(p.Message)
		}
	}

	ctx := context.Background()
	if err := idx.Index(ctx, fullReindex, progress); err != nil {
		return err
	}

	fmt.Println()

	docCount, _ := database.DocumentCount()
	chunkCount, _ := database.ChunkCount()
	fmt.Printf("Index complete: %d documents, %d chunks\n", docCount, chunkCount)

	return nil
}

func runWatch(database *db.DB, cohereClient *cohere.Client, cfg *config.Config) error {
	idx := indexer.New(database, cohereClient, cfg.ObsidianDir)

	watcher, err := indexer.NewWatcher(idx)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nStopping watcher...")
		cancel()
	}()

	return watcher.Start(ctx)
}

func runSearch(database *db.DB, cohereClient *cohere.Client, cfg *config.Config, query string) error {
	searcher := search.New(database, cohereClient)

	ctx := context.Background()
	results, err := searcher.Search(ctx, query)
	if err != nil {
		return err
	}

	model := tui.NewSearchModel(query, cfg.ObsidianDir)

	tuiResults := make([]tui.SearchResult, len(results))
	for i, r := range results {
		tuiResults[i] = tui.SearchResult{
			Rank:    r.Rank,
			Score:   r.Score,
			Path:    r.Path,
			Heading: r.Heading,
			Snippet: r.Content,
			DocID:   r.DocID,
			ChunkID: r.ChunkID,
		}
	}

	initCmd := func() tea.Msg {
		return tui.SearchResultsMsg{Results: tuiResults}
	}

	program := tea.NewProgram(model)

	go func() {
		program.Send(initCmd())
	}()

	if _, err := program.Run(); err != nil {
		return err
	}

	return nil
}

func printUsage() {
	fmt.Println("obsvec - Obsidian Vector Search")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  ofind -q \"search query\"   Search your Obsidian vault")
	fmt.Println("  ofind -index              Index your Obsidian vault")
	fmt.Println("  ofind -index -full        Full reindex (ignore cache)")
	fmt.Println("  ofind -watch              Watch for changes and auto-index")
	fmt.Println("  ofind -setup              Run setup wizard")
	fmt.Println()
}
