package llm

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/startower-observability/blackcat/internal/agentapi"
)

// mockDiscoveryAdapter implements ProviderDiscoveryAdapter for testing.
type mockDiscoveryAdapter struct {
	name      string
	models    []agentapi.ProviderModelRecord
	err       error
	callCount int
	mu        sync.Mutex
}

func (m *mockDiscoveryAdapter) DiscoverModels(_ context.Context) ([]agentapi.ProviderModelRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return m.models, nil
}

func (m *mockDiscoveryAdapter) ProviderName() string {
	return m.name
}

func (m *mockDiscoveryAdapter) calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func TestProviderCatalogRefreshAndServe(t *testing.T) {
	adapter := &mockDiscoveryAdapter{
		name: "testprovider",
		models: []agentapi.ProviderModelRecord{
			{
				ID:            "model-a",
				Name:          "Model A",
				Aliases:       []string{"a"},
				ContextWindow: 128000,
				MaxOutput:     4096, // should NOT appear in ProviderModelView
				Modalities:    []string{"text"},
			},
			{
				ID:            "model-b",
				Name:          "Model B",
				ContextWindow: 32000,
				MaxOutput:     8192, // should NOT appear in ProviderModelView
				Modalities:    []string{"text", "image"},
			},
		},
	}

	cache := NewProviderCatalogCache(1 * time.Minute)
	cache.RegisterAdapter(adapter)

	entry := cache.Get(context.Background(), "testprovider")

	// Verify models returned
	if len(entry.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(entry.Models))
	}

	// Verify source is live
	if entry.Freshness.Source != agentapi.SourceLive {
		t.Errorf("expected source %q, got %q", agentapi.SourceLive, entry.Freshness.Source)
	}

	// Verify models are ProviderModelView (no MaxOutput field)
	for _, mv := range entry.Models {
		if mv.ID == "" {
			t.Error("model view ID should not be empty")
		}
		if mv.Freshness.Source != agentapi.SourceLive {
			t.Errorf("model %q freshness source: expected %q, got %q", mv.ID, agentapi.SourceLive, mv.Freshness.Source)
		}
	}

	// Verify specific model fields
	if entry.Models[0].Name != "Model A" {
		t.Errorf("expected model name %q, got %q", "Model A", entry.Models[0].Name)
	}
	if entry.Models[0].ContextWindow != 128000 {
		t.Errorf("expected context window 128000, got %d", entry.Models[0].ContextWindow)
	}
	if len(entry.Models[1].Modalities) != 2 {
		t.Errorf("expected 2 modalities for model-b, got %d", len(entry.Models[1].Modalities))
	}

	// Verify provider name
	if entry.Provider != "testprovider" {
		t.Errorf("expected provider %q, got %q", "testprovider", entry.Provider)
	}

	// Second call within TTL should use cache (no additional adapter call)
	entry2 := cache.Get(context.Background(), "testprovider")
	if adapter.calls() != 1 {
		t.Errorf("expected 1 adapter call (cached), got %d", adapter.calls())
	}
	if len(entry2.Models) != 2 {
		t.Fatalf("cached result: expected 2 models, got %d", len(entry2.Models))
	}
}

func TestProviderCatalogReturnsCachedStaleOnFailure(t *testing.T) {
	adapter := &mockDiscoveryAdapter{
		name: "failprovider",
		models: []agentapi.ProviderModelRecord{
			{
				ID:   "model-x",
				Name: "Model X",
			},
		},
	}

	// Use very short TTL so cache expires quickly
	cache := NewProviderCatalogCache(1 * time.Millisecond)
	cache.RegisterAdapter(adapter)

	// First call succeeds
	entry := cache.Get(context.Background(), "failprovider")
	if entry.Freshness.Source != agentapi.SourceLive {
		t.Fatalf("first call: expected source %q, got %q", agentapi.SourceLive, entry.Freshness.Source)
	}
	if len(entry.Models) != 1 {
		t.Fatalf("first call: expected 1 model, got %d", len(entry.Models))
	}

	// Wait for TTL to expire
	time.Sleep(5 * time.Millisecond)

	// Make adapter fail on next call
	adapter.mu.Lock()
	adapter.err = errors.New("provider API unavailable")
	adapter.mu.Unlock()

	// Second call should return stale data with error
	entry2 := cache.Get(context.Background(), "failprovider")

	if entry2.Freshness.Source != agentapi.SourceCachedStale {
		t.Errorf("stale call: expected source %q, got %q", agentapi.SourceCachedStale, entry2.Freshness.Source)
	}
	if entry2.Freshness.LastAttemptError == "" {
		t.Error("stale call: expected non-empty LastAttemptError")
	}
	if entry2.Freshness.LastAttemptError != "provider API unavailable" {
		t.Errorf("stale call: expected error %q, got %q", "provider API unavailable", entry2.Freshness.LastAttemptError)
	}
	// Stale data should still have the old model
	if len(entry2.Models) != 1 {
		t.Fatalf("stale call: expected 1 model, got %d", len(entry2.Models))
	}
	if entry2.Models[0].ID != "model-x" {
		t.Errorf("stale call: expected model ID %q, got %q", "model-x", entry2.Models[0].ID)
	}
}

func TestProviderCatalogConcurrentAccess(t *testing.T) {
	adapter := &mockDiscoveryAdapter{
		name: "concurrent",
		models: []agentapi.ProviderModelRecord{
			{ID: "model-1", Name: "Model 1"},
			{ID: "model-2", Name: "Model 2"},
		},
	}

	cache := NewProviderCatalogCache(50 * time.Millisecond)
	cache.RegisterAdapter(adapter)

	var wg sync.WaitGroup
	const goroutines = 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			entry := cache.Get(context.Background(), "concurrent")
			if len(entry.Models) != 2 {
				t.Errorf("expected 2 models, got %d", len(entry.Models))
			}
			if entry.Provider != "concurrent" {
				t.Errorf("expected provider %q, got %q", "concurrent", entry.Provider)
			}
		}()
	}

	wg.Wait()

	// Verify no panics occurred and data is consistent
	entry := cache.Get(context.Background(), "concurrent")
	if entry.Freshness.Source != agentapi.SourceLive {
		t.Errorf("final check: expected source %q, got %q", agentapi.SourceLive, entry.Freshness.Source)
	}
}

func TestProviderCatalogNoAdapter(t *testing.T) {
	cache := NewProviderCatalogCache(1 * time.Minute)

	entry := cache.Get(context.Background(), "nonexistent")

	if entry.Provider != "nonexistent" {
		t.Errorf("expected provider %q, got %q", "nonexistent", entry.Provider)
	}
	if entry.Freshness.Source != agentapi.SourceUnknown {
		t.Errorf("expected source %q, got %q", agentapi.SourceUnknown, entry.Freshness.Source)
	}
	if len(entry.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(entry.Models))
	}
}

func TestProviderCatalogProviders(t *testing.T) {
	cache := NewProviderCatalogCache(1 * time.Minute)

	a1 := &mockDiscoveryAdapter{name: "alpha"}
	a2 := &mockDiscoveryAdapter{name: "beta"}

	cache.RegisterAdapter(a1)
	cache.RegisterAdapter(a2)

	providers := cache.Providers()
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}

	found := map[string]bool{}
	for _, p := range providers {
		found[p] = true
	}
	if !found["alpha"] || !found["beta"] {
		t.Errorf("expected providers alpha and beta, got %v", providers)
	}
}

func TestProviderCatalogRefreshForced(t *testing.T) {
	callModels := []agentapi.ProviderModelRecord{
		{ID: "v1", Name: "Version 1"},
	}
	adapter := &mockDiscoveryAdapter{
		name:   "refreshable",
		models: callModels,
	}

	cache := NewProviderCatalogCache(10 * time.Minute) // long TTL
	cache.RegisterAdapter(adapter)

	// Initial Get
	entry := cache.Get(context.Background(), "refreshable")
	if len(entry.Models) != 1 || entry.Models[0].ID != "v1" {
		t.Fatalf("initial: expected v1, got %v", entry.Models)
	}

	// Update adapter models
	adapter.mu.Lock()
	adapter.models = []agentapi.ProviderModelRecord{
		{ID: "v2", Name: "Version 2"},
		{ID: "v3", Name: "Version 3"},
	}
	adapter.mu.Unlock()

	// Get should still return cached (TTL not expired)
	entry2 := cache.Get(context.Background(), "refreshable")
	if len(entry2.Models) != 1 {
		t.Fatalf("cached: expected 1 model (cached), got %d", len(entry2.Models))
	}

	// Refresh forces update regardless of TTL
	entry3 := cache.Refresh(context.Background(), "refreshable")
	if len(entry3.Models) != 2 {
		t.Fatalf("refreshed: expected 2 models, got %d", len(entry3.Models))
	}
	if entry3.Models[0].ID != "v2" {
		t.Errorf("refreshed: expected v2, got %q", entry3.Models[0].ID)
	}
	if entry3.Freshness.Source != agentapi.SourceLive {
		t.Errorf("refreshed: expected source %q, got %q", agentapi.SourceLive, entry3.Freshness.Source)
	}
}
