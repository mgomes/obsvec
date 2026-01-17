package tui

type SetupSubmitMsg struct {
	APIKey      string
	ObsidianDir string
}

type SetupErrorMsg struct {
	Error string
}

type SearchResultsMsg struct {
	Results []SearchResult
}

type SearchErrorMsg struct {
	Error string
}

type SearchResult struct {
	Rank     int
	Score    float64
	Path     string
	Heading  string
	Snippet  string
	DocID    int64
	ChunkID  int64
}
