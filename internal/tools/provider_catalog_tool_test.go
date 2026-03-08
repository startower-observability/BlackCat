package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/startower-observability/blackcat/internal/agentapi"
	"github.com/startower-observability/blackcat/internal/llm"
)

// stubDiscoveryAdapter implements llm.ProviderDiscoveryAdapter for tests.
type stubDiscoveryAdapter struct {
	name   string
	models []agentapi.ProviderModelRecord
	err    error
}

func (s *stubDiscoveryAdapter) ProviderName() string { return s.name }
func (s *stubDiscoveryAdapter) DiscoverModels(_ context.Context) ([]agentapi.ProviderModelRecord, error) {
	return s.models, s.err
}

func newTestCache(adapters ...llm.ProviderDiscoveryAdapter) *llm.ProviderCatalogCache {
	cache := llm.NewProviderCatalogCache(1 * time.Hour)
	for _, a := range adapters {
		cache.RegisterAdapter(a)
	}
	// Pre-warm: call Get for each adapter to populate the cache
	ctx := context.Background()
	for _, a := range adapters {
		cache.Get(ctx, a.ProviderName())
	}
	return cache
}

func TestProviderCatalogToolAnswersGitHubModelsQuery(t *testing.T) {
	ctx := context.Background()

	copilotAdapter := &stubDiscoveryAdapter{
		name: "copilot",
		models: []agentapi.ProviderModelRecord{
			{ID: "gpt-4o", Name: "GPT-4o", ContextWindow: 128000, Modalities: []string{"text", "image"}},
			{ID: "gpt-4o-mini", Name: "GPT-4o Mini", ContextWindow: 128000, Modalities: []string{"text"}},
			{ID: "o1-preview", Name: "O1 Preview", ContextWindow: 128000},
		},
	}

	cache := newTestCache(copilotAdapter)
	tool := NewProviderCatalogTool(cache)

	// Verify tool metadata
	if tool.Name() != "provider_catalog" {
		t.Fatalf("Name() = %q, want provider_catalog", tool.Name())
	}

	// --- Test github-models alias maps to copilot ---
	result, err := tool.Execute(ctx, json.RawMessage(`{"provider":"github-models"}`))
	if err != nil {
		t.Fatalf("Execute(github-models) error: %v", err)
	}

	if !strings.Contains(result, "gpt-4o") {
		t.Error("github-models query should return copilot models (gpt-4o)")
	}
	if !strings.Contains(result, "copilot") {
		t.Error("github-models query should show provider as 'copilot'")
	}

	// --- Test github_models alias (underscore variant) ---
	result2, err := tool.Execute(ctx, json.RawMessage(`{"provider":"github_models"}`))
	if err != nil {
		t.Fatalf("Execute(github_models) error: %v", err)
	}
	if !strings.Contains(result2, "gpt-4o") {
		t.Error("github_models query should also return copilot models")
	}

	// --- Test direct copilot query ---
	resultDirect, err := tool.Execute(ctx, json.RawMessage(`{"provider":"copilot"}`))
	if err != nil {
		t.Fatalf("Execute(copilot) error: %v", err)
	}
	if !strings.Contains(resultDirect, "gpt-4o") {
		t.Error("copilot query should return copilot models")
	}

	// --- Verify output is valid JSON ---
	var entries []agentapi.CatalogEntry
	if err := json.Unmarshal([]byte(resultDirect), &entries); err != nil {
		t.Fatalf("output should be valid JSON: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 catalog entry, got %d", len(entries))
	}
	if entries[0].Provider != "copilot" {
		t.Errorf("provider = %q, want copilot", entries[0].Provider)
	}
	if len(entries[0].Models) != 3 {
		t.Errorf("model count = %d, want 3", len(entries[0].Models))
	}
}

func TestProviderCatalogToolAllProviders(t *testing.T) {
	ctx := context.Background()

	openaiAdapter := &stubDiscoveryAdapter{
		name: "openai",
		models: []agentapi.ProviderModelRecord{
			{ID: "gpt-4o", Name: "GPT-4o"},
		},
	}
	geminiAdapter := &stubDiscoveryAdapter{
		name: "gemini",
		models: []agentapi.ProviderModelRecord{
			{ID: "gemini-pro", Name: "Gemini Pro"},
		},
	}

	cache := newTestCache(openaiAdapter, geminiAdapter)
	tool := NewProviderCatalogTool(cache)

	// Query all providers (no provider param)
	result, err := tool.Execute(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if !strings.Contains(result, "openai") {
		t.Error("all-provider query should contain openai")
	}
	if !strings.Contains(result, "gemini") {
		t.Error("all-provider query should contain gemini")
	}
}

func TestProviderCatalogToolUnknownProvider(t *testing.T) {
	ctx := context.Background()

	cache := newTestCache() // no adapters
	tool := NewProviderCatalogTool(cache)

	result, err := tool.Execute(ctx, json.RawMessage(`{"provider":"nonexistent"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if !strings.Contains(result, "No catalog available for provider: nonexistent") {
		t.Errorf("expected 'No catalog available' message, got: %s", result)
	}
}

func TestProviderCatalogToolNilCache(t *testing.T) {
	ctx := context.Background()

	tool := NewProviderCatalogTool(nil)

	result, err := tool.Execute(ctx, json.RawMessage(`{"provider":"openai"}`))
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if !strings.Contains(result, "No catalog available") {
		t.Errorf("expected 'No catalog available' message, got: %s", result)
	}
}
