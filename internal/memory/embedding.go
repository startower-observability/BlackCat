package memory

import (
	"context"
	"errors"
	"math"

	openai "github.com/sashabaranov/go-openai"
)

// ErrNoEmbeddingProvider is returned when no embedding provider is configured
// (i.e., the EmbeddingClient is nil or was created without an API key).
var ErrNoEmbeddingProvider = errors.New("embedding: no provider configured")

// DefaultEmbeddingModel is the default model used for generating embeddings.
const DefaultEmbeddingModel = "text-embedding-3-small"

// EmbeddingClient wraps the OpenAI Embeddings API to generate vector
// representations of text. It is nil-safe: calling methods on a nil
// *EmbeddingClient returns ErrNoEmbeddingProvider.
type EmbeddingClient struct {
	client *openai.Client
	model  string
}

// NewEmbeddingClient creates a new EmbeddingClient configured for the given
// provider. If apiKey is empty, nil is returned — callers should check for nil
// before use. The baseURL parameter allows pointing at OpenAI-compatible
// endpoints; pass "" to use the default OpenAI API URL. The model parameter
// selects the embedding model; pass "" to use DefaultEmbeddingModel.
func NewEmbeddingClient(apiKey, baseURL, model string) *EmbeddingClient {
	if apiKey == "" {
		return nil
	}

	if model == "" {
		model = DefaultEmbeddingModel
	}

	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}

	return &EmbeddingClient{
		client: openai.NewClientWithConfig(cfg),
		model:  model,
	}
}

// Embed generates embeddings for a batch of texts. Returns one []float32
// vector per input text, in the same order as the input slice.
// Returns ErrNoEmbeddingProvider if called on a nil receiver.
func (ec *EmbeddingClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if ec == nil {
		return nil, ErrNoEmbeddingProvider
	}

	resp, err := ec.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(ec.model),
		Input: texts,
	})
	if err != nil {
		return nil, err
	}

	embeddings := make([][]float32, len(resp.Data))
	for i, item := range resp.Data {
		embeddings[i] = item.Embedding
	}

	return embeddings, nil
}

// EmbedSingle is a convenience method that embeds a single text string.
// Returns ErrNoEmbeddingProvider if called on a nil receiver.
func (ec *EmbeddingClient) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	if ec == nil {
		return nil, ErrNoEmbeddingProvider
	}

	results, err := ec.Embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, errors.New("embedding: no results returned")
	}

	return results[0], nil
}

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns a value in [-1, 1] where 1 means identical direction, 0 means
// orthogonal, and -1 means opposite direction.
//
// Edge cases: returns 0.0 for zero-length slices, mismatched lengths,
// or all-zeros vectors.
func CosineSimilarity(a, b []float32) float32 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0.0
	}

	var dot, normA, normB float64
	for i := range a {
		ai := float64(a[i])
		bi := float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}
