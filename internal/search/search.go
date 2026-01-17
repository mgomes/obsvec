package search

import (
	"context"
	"fmt"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/mgomes/obsvec/internal/cohere"
	"github.com/mgomes/obsvec/internal/db"
)

const (
	vectorSearchLimit = 20
	rerankTopN        = 10
)

type Searcher struct {
	db     *db.DB
	cohere *cohere.Client
}

type Result struct {
	Rank      int
	Score     float64
	Path      string
	Heading   string
	Content   string
	StartLine int
	EndLine   int
	DocID     int64
	ChunkID   int64
}

func New(database *db.DB, cohereClient *cohere.Client) *Searcher {
	return &Searcher{
		db:     database,
		cohere: cohereClient,
	}
}

func (s *Searcher) Search(ctx context.Context, query string) ([]Result, error) {
	queryEmb, err := s.cohere.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	embBytes, err := sqlite_vec.SerializeFloat32(queryEmb)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize query embedding: %w", err)
	}

	candidates, err := s.db.SearchSimilar(embBytes, vectorSearchLimit)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	docs := make([]string, len(candidates))
	for i, c := range candidates {
		docs[i] = c.Content
	}

	rerankResults, err := s.cohere.Rerank(ctx, query, docs, rerankTopN)
	if err != nil {
		return nil, fmt.Errorf("rerank failed: %w", err)
	}

	results := make([]Result, len(rerankResults))
	for i, rr := range rerankResults {
		c := candidates[rr.Index]
		results[i] = Result{
			Rank:      i + 1,
			Score:     rr.Score,
			Path:      c.Path,
			Heading:   c.Heading,
			Content:   c.Content,
			StartLine: c.StartLine,
			EndLine:   c.EndLine,
			DocID:     c.DocID,
			ChunkID:   c.ID,
		}
	}

	return results, nil
}

func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen-3] + "..."
}
