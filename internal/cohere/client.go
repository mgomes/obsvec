package cohere

import (
	"context"
	"errors"
	"fmt"

	cohere "github.com/cohere-ai/cohere-go/v2"
	cohereclient "github.com/cohere-ai/cohere-go/v2/client"
)

type Client struct {
	client     *cohereclient.Client
	embedModel string
	rerankModel string
	embedDim   int
}

type EmbeddingResult struct {
	Embedding []float32
}

type RerankResult struct {
	Index int
	Score float64
}

func NewClient(apiKey, embedModel, rerankModel string, embedDim int) *Client {
	client := cohereclient.NewClient(cohereclient.WithToken(apiKey))
	return &Client{
		client:      client,
		embedModel:  embedModel,
		rerankModel: rerankModel,
		embedDim:    embedDim,
	}
}

func (c *Client) ValidateAPIKey(ctx context.Context) error {
	_, err := c.client.Models.List(ctx, &cohere.ModelsListRequest{})
	if err != nil {
		return fmt.Errorf("invalid API key: %w", err)
	}
	return nil
}

func (c *Client) EmbedDocuments(ctx context.Context, texts []string) ([]EmbeddingResult, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	embeddings, err := c.embed(ctx, texts, cohere.EmbedInputTypeSearchDocument)
	if err != nil {
		if errors.Is(err, errNoEmbeddings) {
			return nil, err
		}
		return nil, fmt.Errorf("embed request failed: %w", err)
	}

	results := make([]EmbeddingResult, len(embeddings))
	for i, emb := range embeddings {
		results[i] = EmbeddingResult{
			Embedding: emb,
		}
	}

	return results, nil
}

func (c *Client) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	embeddings, err := c.embed(ctx, []string{query}, cohere.EmbedInputTypeSearchQuery)
	if err != nil {
		if errors.Is(err, errNoEmbeddings) {
			return nil, fmt.Errorf("no embedding returned")
		}
		return nil, fmt.Errorf("embed query failed: %w", err)
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return embeddings[0], nil
}

func (c *Client) Rerank(ctx context.Context, query string, documents []string, topN int) ([]RerankResult, error) {
	if len(documents) == 0 {
		return nil, nil
	}

	resp, err := c.client.V2.Rerank(ctx, &cohere.V2RerankRequest{
		Model:     c.rerankModel,
		Query:     query,
		Documents: documents,
		TopN:      &topN,
	})
	if err != nil {
		return nil, fmt.Errorf("rerank request failed: %w", err)
	}

	results := make([]RerankResult, len(resp.Results))
	for i, r := range resp.Results {
		results[i] = RerankResult{
			Index: r.Index,
			Score: r.RelevanceScore,
		}
	}

	return results, nil
}

func float64sToFloat32s(f64s []float64) []float32 {
	f32s := make([]float32, len(f64s))
	for i, v := range f64s {
		f32s[i] = float32(v)
	}
	return f32s
}

var errNoEmbeddings = errors.New("no embeddings returned")

func (c *Client) embed(ctx context.Context, texts []string, inputType cohere.EmbedInputType) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	embeddingTypes := []cohere.EmbeddingType{cohere.EmbeddingTypeFloat}
	outputDim := c.embedDim

	resp, err := c.client.V2.Embed(ctx, &cohere.V2EmbedRequest{
		Texts:           texts,
		Model:           c.embedModel,
		InputType:       inputType,
		EmbeddingTypes:  embeddingTypes,
		OutputDimension: &outputDim,
	})
	if err != nil {
		return nil, err
	}

	if resp.Embeddings == nil || resp.Embeddings.Float == nil {
		return nil, errNoEmbeddings
	}

	results := make([][]float32, len(resp.Embeddings.Float))
	for i, emb := range resp.Embeddings.Float {
		results[i] = float64sToFloat32s(emb)
	}

	return results, nil
}
