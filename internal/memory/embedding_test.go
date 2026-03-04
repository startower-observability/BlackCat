package memory

import (
	"context"
	"errors"
	"math"
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// CosineSimilarity tests
// ---------------------------------------------------------------------------

func TestCosineSimilarity_IdenticalVectors(t *testing.T) {
	a := []float32{1, 0, 0}
	got := CosineSimilarity(a, a)
	if diff := math.Abs(float64(got) - 1.0); diff > 1e-6 {
		t.Errorf("CosineSimilarity(a, a) = %f; want ≈ 1.0", got)
	}
}

func TestCosineSimilarity_OrthogonalVectors(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	got := CosineSimilarity(a, b)
	if diff := math.Abs(float64(got)); diff > 1e-6 {
		t.Errorf("CosineSimilarity([1,0,0], [0,1,0]) = %f; want ≈ 0.0", got)
	}
}

func TestCosineSimilarity_OppositeVectors(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{-1, 0, 0}
	got := CosineSimilarity(a, b)
	if diff := math.Abs(float64(got) - (-1.0)); diff > 1e-6 {
		t.Errorf("CosineSimilarity([1,0,0], [-1,0,0]) = %f; want ≈ -1.0", got)
	}
}

func TestCosineSimilarity_Symmetric(t *testing.T) {
	a := []float32{0.3, 0.7, 0.1}
	b := []float32{0.5, 0.2, 0.9}
	ab := CosineSimilarity(a, b)
	ba := CosineSimilarity(b, a)
	if ab != ba {
		t.Errorf("CosineSimilarity not symmetric: (%f, %f) != (%f, %f)", ab, ba, ab, ba)
	}
}

func TestCosineSimilarity_EmptyVectors(t *testing.T) {
	got := CosineSimilarity([]float32{}, []float32{})
	if got != 0.0 {
		t.Errorf("CosineSimilarity([], []) = %f; want 0.0", got)
	}
}

func TestCosineSimilarity_NilVectors(t *testing.T) {
	got := CosineSimilarity(nil, nil)
	if got != 0.0 {
		t.Errorf("CosineSimilarity(nil, nil) = %f; want 0.0", got)
	}
}

func TestCosineSimilarity_MismatchedLengths(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{1, 0, 0}
	got := CosineSimilarity(a, b)
	if got != 0.0 {
		t.Errorf("CosineSimilarity with mismatched lengths = %f; want 0.0", got)
	}
}

func TestCosineSimilarity_AllZeros(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 0, 0}
	got := CosineSimilarity(a, b)
	if got != 0.0 {
		t.Errorf("CosineSimilarity with zero vector = %f; want 0.0", got)
	}
}

func TestCosineSimilarity_BothZeros(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{0, 0, 0}
	got := CosineSimilarity(a, b)
	if got != 0.0 {
		t.Errorf("CosineSimilarity with both zero vectors = %f; want 0.0", got)
	}
}

// ---------------------------------------------------------------------------
// EmbeddingClient nil-safety tests
// ---------------------------------------------------------------------------

func TestNewEmbeddingClient_EmptyAPIKey(t *testing.T) {
	client := NewEmbeddingClient("", "", "")
	if client != nil {
		t.Error("NewEmbeddingClient with empty API key should return nil")
	}
}

func TestNilEmbeddingClient_Embed(t *testing.T) {
	var client *EmbeddingClient // nil
	_, err := client.Embed(context.Background(), []string{"hello"})
	if !errors.Is(err, ErrNoEmbeddingProvider) {
		t.Errorf("nil EmbeddingClient.Embed() error = %v; want ErrNoEmbeddingProvider", err)
	}
}

func TestNilEmbeddingClient_EmbedSingle(t *testing.T) {
	var client *EmbeddingClient // nil
	_, err := client.EmbedSingle(context.Background(), "hello")
	if !errors.Is(err, ErrNoEmbeddingProvider) {
		t.Errorf("nil EmbeddingClient.EmbedSingle() error = %v; want ErrNoEmbeddingProvider", err)
	}
}

func TestNewEmbeddingClient_DefaultModel(t *testing.T) {
	client := NewEmbeddingClient("test-key", "", "")
	if client == nil {
		t.Fatal("NewEmbeddingClient with valid API key should not return nil")
	}
	if client.model != DefaultEmbeddingModel {
		t.Errorf("model = %q; want %q", client.model, DefaultEmbeddingModel)
	}
}

func TestNewEmbeddingClient_CustomModel(t *testing.T) {
	client := NewEmbeddingClient("test-key", "", "text-embedding-3-large")
	if client == nil {
		t.Fatal("NewEmbeddingClient with valid API key should not return nil")
	}
	if client.model != "text-embedding-3-large" {
		t.Errorf("model = %q; want %q", client.model, "text-embedding-3-large")
	}
}

// ---------------------------------------------------------------------------
// Live API tests (skipped without OPENAI_API_KEY)
// ---------------------------------------------------------------------------

func TestEmbedSingle_LiveAPI(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set; skipping live API test")
	}

	client := NewEmbeddingClient(apiKey, "", "")
	vec, err := client.EmbedSingle(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("EmbedSingle() error: %v", err)
	}
	if len(vec) == 0 {
		t.Fatal("EmbedSingle() returned empty vector")
	}
	t.Logf("embedding dimensions: %d", len(vec))
}

func TestEmbed_LiveAPI(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set; skipping live API test")
	}

	client := NewEmbeddingClient(apiKey, "", "")
	texts := []string{"hello world", "goodbye world"}
	vecs, err := client.Embed(context.Background(), texts)
	if err != nil {
		t.Fatalf("Embed() error: %v", err)
	}
	if len(vecs) != len(texts) {
		t.Fatalf("Embed() returned %d vectors; want %d", len(vecs), len(texts))
	}
	for i, vec := range vecs {
		if len(vec) == 0 {
			t.Errorf("Embed() vector[%d] is empty", i)
		}
	}

	// Similar texts should have high cosine similarity
	sim := CosineSimilarity(vecs[0], vecs[1])
	t.Logf("similarity between 'hello world' and 'goodbye world': %f", sim)
	if sim < 0.5 {
		t.Errorf("expected similarity > 0.5 for related texts; got %f", sim)
	}
}
