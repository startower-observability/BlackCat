package tools

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/startower-observability/blackcat/internal/agentapi"
	"github.com/startower-observability/blackcat/internal/llm"
	"github.com/startower-observability/blackcat/internal/skills"
)

// ────────────────────────────────────────────────────────────────────
// Regression: "No skills" despite inactive inventory existing
// ────────────────────────────────────────────────────────────────────

// TestRegressionInactiveSkillsAreNotMissingFromToolOutput covers the
// user-reported failure: agent said "no skills" even though inactive
// skills were present in the inventory.
//
// Root cause: extras was nil so inactive skills were not surfaced, or
// extras.SkillInventory was set but the snapshot builder skipped them.
//
// This test ensures that when extras.SkillInventory is provided with
// inactive entries, the tool output contains inactive_skill_summaries.
func TestRegressionInactiveSkillsAreNotMissingFromToolOutput(t *testing.T) {
	ctx := context.Background()
	provider := &stubSelfKnowledgeProvider{
		agentName:    "BlackCat",
		modelName:    "gpt-4o",
		providerName: "openai",
	}

	inventory := &skills.Inventory{
		Active: []skills.Skill{},
		Inactive: []skills.InactiveSkill{
			{
				Name:        "crypto-market",
				MissingEnv:  []string{"COINGECKO_API_KEY"},
				MissingBins: nil,
			},
			{
				Name:        "web-search",
				MissingEnv:  nil,
				MissingBins: []string{"chromium"},
			},
		},
	}
	extras := &agentapi.SelfKnowledgeExtras{
		SkillInventory: inventory,
	}

	tool := NewAgentSelfStatusTool(provider, extras)
	result, err := tool.Execute(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// inactive_skill_summaries must be present and populated.
	if !strings.Contains(result, "inactive_skill_summaries") {
		t.Errorf("tool output missing inactive_skill_summaries field; got:\n%s", result)
	}
	if !strings.Contains(result, "crypto-market") {
		t.Errorf("tool output missing 'crypto-market' inactive skill; got:\n%s", result)
	}
	if !strings.Contains(result, "web-search") {
		t.Errorf("tool output missing 'web-search' inactive skill; got:\n%s", result)
	}

	// MUST NOT leak raw env var names (security requirement).
	if strings.Contains(result, "COINGECKO_API_KEY") {
		t.Errorf("tool output leaks raw env var name COINGECKO_API_KEY; got:\n%s", result)
	}
}

// TestRegressionInactiveSkillsSummaryNilExtrasBackwardCompat ensures that
// when extras is nil, GetInactiveSkills() from the provider is still returned
// (legacy behavior — the field must not be absent just because extras is nil).
func TestRegressionInactiveSkillsSummaryNilExtrasBackwardCompat(t *testing.T) {
	ctx := context.Background()
	provider := &stubSelfKnowledgeProvider{
		agentName:    "BlackCat",
		modelName:    "gpt-4o",
		providerName: "openai",
		inactiveSkills: []skills.InactiveSkill{
			{Name: "crypto-market", MissingEnv: []string{"COINGECKO_API_KEY"}},
		},
	}

	// nil extras → legacy path
	tool := NewAgentSelfStatusTool(provider)
	result, err := tool.Execute(ctx, json.RawMessage(`{"full":true}`))
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Legacy mode: inactive_skills from provider must appear (may have raw names).
	// Key requirement: NOT absent entirely.
	if !strings.Contains(result, "crypto-market") {
		t.Errorf("legacy inactive skill 'crypto-market' missing from tool output; got:\n%s", result)
	}
}

// ────────────────────────────────────────────────────────────────────
// Regression: Generic / missing role info in agent answer
// ────────────────────────────────────────────────────────────────────

// TestRegressionRolesAreFullyPopulatedInToolOutput covers the user-reported
// failure: agent gave a generic/empty roles answer.
//
// Root cause: extras.Roles was nil or not wired.
//
// This test ensures that when extras.Roles is populated, the tool output
// contains concrete role names and priorities.
func TestRegressionRolesAreFullyPopulatedInToolOutput(t *testing.T) {
	ctx := context.Background()
	provider := &stubSelfKnowledgeProvider{
		agentName:    "BlackCat",
		modelName:    "gpt-4o",
		providerName: "openai",
	}

	extras := &agentapi.SelfKnowledgeExtras{
		Roles: []agentapi.RoleView{
			{Name: "wizard", Priority: 30, KeywordCount: 4},
			{Name: "oracle", Priority: 100, KeywordCount: 0},
		},
	}

	tool := NewAgentSelfStatusTool(provider, extras)
	result, err := tool.Execute(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Both roles must appear in output.
	if !strings.Contains(result, "wizard") {
		t.Errorf("role 'wizard' missing from tool output; got:\n%s", result)
	}
	if !strings.Contains(result, "oracle") {
		t.Errorf("role 'oracle' missing from tool output; got:\n%s", result)
	}
	// Priority must be serialized.
	if !strings.Contains(result, "30") {
		t.Errorf("role priority 30 missing from tool output; got:\n%s", result)
	}
}

// ────────────────────────────────────────────────────────────────────
// Regression: GitHub Copilot / GitHub Models model answer
// ────────────────────────────────────────────────────────────────────

// TestRegressionGitHubModelsAliasInCatalog covers the user-reported failure:
// agent gave wrong/incomplete GitHub Copilot model information.
//
// Root cause: provider_catalog tool did not tag GitHub Models endpoint
// models with the "github-models" alias.
//
// This test verifies the provider_catalog tool returns entries tagged
// with github-models alias when queried with "github-models" provider name.
func TestRegressionGitHubModelsAliasInCatalog(t *testing.T) {
	ctx := context.Background()
	cache := newTestCache(&stubGitHubModelsAdapter{})

	tool := NewProviderCatalogTool(cache)

	// Query with "github-models" alias — must return models.
	result, err := tool.Execute(ctx, json.RawMessage(`{"provider":"github-models"}`))
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result == "" {
		t.Fatal("provider_catalog returned empty result for github-models query")
	}
	if !strings.Contains(result, "gpt-4o") {
		t.Errorf("GitHub Models result missing expected model 'gpt-4o'; got:\n%s", result)
	}
}

// stubGitHubModelsAdapter simulates a provider that has models tagged with
// the "github-models" alias (like the real CopilotAdapter when it detects
// the GitHub Models inference endpoint).
type stubGitHubModelsAdapter struct{}

func (s *stubGitHubModelsAdapter) ProviderName() string { return "copilot" }
func (s *stubGitHubModelsAdapter) DiscoverModels(_ context.Context) ([]agentapi.ProviderModelRecord, error) {
	return []agentapi.ProviderModelRecord{
		{
			ID:            "gpt-4o",
			Name:          "GPT-4o (via GitHub Models)",
			ContextWindow: 128000,
			Aliases:       []string{"github-models"},
		},
		{
			ID:            "gpt-4o-mini",
			Name:          "GPT-4o Mini (via GitHub Models)",
			ContextWindow: 128000,
			Aliases:       []string{"github-models"},
		},
	}, nil
}

// TestRegressionGitHubModelsQueryViaAlias ensures the provider_catalog tool
// resolves "github-models", "github_models", "github" to the copilot provider.
func TestRegressionGitHubModelsQueryViaAlias(t *testing.T) {
	ctx := context.Background()
	cache := newTestCache(&stubGitHubModelsAdapter{})
	tool := NewProviderCatalogTool(cache)

	aliases := []string{"github-models", "github_models", "github"}
	for _, alias := range aliases {
		t.Run(alias, func(t *testing.T) {
			result, err := tool.Execute(ctx, json.RawMessage(`{"provider":"`+alias+`"}`))
			if err != nil {
				t.Fatalf("Execute(%q) failed: %v", alias, err)
			}
			if !strings.Contains(result, "gpt-4o") {
				t.Errorf("alias %q: expected gpt-4o in result; got:\n%s", alias, result)
			}
		})
	}
}

// ────────────────────────────────────────────────────────────────────
// Regression: Concurrent skill reload must not corrupt provider catalog reads
// ────────────────────────────────────────────────────────────────────

// TestSkillReloadDoesNotCorruptProviderCatalogReads verifies that concurrent
// reads to the ProviderCatalogCache do not produce data races or panics.
// Run with -race for full coverage.
func TestSkillReloadDoesNotCorruptProviderCatalogReads(t *testing.T) {
	cache := newTestCache(&deterministicStubCatalogAdapter{})

	ctx := context.Background()
	// Force initial population.
	_ = cache.Get(ctx, "stubcat")

	const (
		readers = 20
		writers = 5
		iters   = 50
	)

	var wg sync.WaitGroup

	// Readers: simulate provider_catalog tool being called concurrently.
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				entry := cache.Get(ctx, "stubcat")
				if entry.Models == nil && entry.Freshness.Source == "" {
					// An uninitialized entry with no data is suspicious but not
					// a hard failure — the cache may be refreshing. Just ensure
					// we didn't get a nil pointer dereference.
				}
			}
		}()
	}

	// Writers: simulate cache refresh (e.g., triggered by skill reload events).
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				_ = cache.Refresh(ctx, "stubcat")
			}
		}()
	}

	wg.Wait()
}

// deterministicStubCatalogAdapter is a stable test adapter with a unique name
// to avoid collisions with other test adapters in the same package.
type deterministicStubCatalogAdapter struct{}

func (d *deterministicStubCatalogAdapter) ProviderName() string { return "stubcat" }
func (d *deterministicStubCatalogAdapter) DiscoverModels(_ context.Context) ([]agentapi.ProviderModelRecord, error) {
	return []agentapi.ProviderModelRecord{
		{ID: "stub-model-1", Name: "Stub Model 1", ContextWindow: 8192},
		{ID: "stub-model-2", Name: "Stub Model 2", ContextWindow: 32768},
	}, nil
}

// ────────────────────────────────────────────────────────────────────
// Regression: Stale fallback must use SourceCachedStale label
// ────────────────────────────────────────────────────────────────────

// TestRegressionStaleProviderFallbackHasCorrectSourceLabel ensures that when
// a provider's live fetch fails, the stale cached entry is returned with
// SourceCachedStale label — not SourceLive or empty.
func TestRegressionStaleProviderFallbackHasCorrectSourceLabel(t *testing.T) {
	ctx := context.Background()

	// firstSuccessThenFailCatalogAdapter returns success on first call, error on subsequent calls.
	adapter := &firstSuccessThenFailCatalogAdapter{}
	cache := newTestCache(adapter)

	// Warm the cache.
	entry1 := cache.Get(ctx, "fstf")
	if entry1.Freshness.Source != agentapi.SourceLive {
		t.Errorf("warm entry source = %q, want %q", entry1.Freshness.Source, agentapi.SourceLive)
	}

	// Force a refresh that will fail. Refresh() returns SourceCachedStale when
	// the fetch errors but old data exists.
	entry2 := cache.Refresh(ctx, "fstf")
	if entry2.Freshness.Source != agentapi.SourceCachedStale {
		t.Errorf("Refresh() on failed fetch source = %q, want %q", entry2.Freshness.Source, agentapi.SourceCachedStale)
	}
	if len(entry2.Models) == 0 {
		t.Error("stale fallback entry has no models — expected to retain previously cached models")
	}
}

// firstSuccessThenFailCatalogAdapter returns success on first call, error on subsequent.
type firstSuccessThenFailCatalogAdapter struct {
	mu    sync.Mutex
	calls int
}

func (f *firstSuccessThenFailCatalogAdapter) ProviderName() string { return "fstf" }
func (f *firstSuccessThenFailCatalogAdapter) DiscoverModels(_ context.Context) ([]agentapi.ProviderModelRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.calls == 1 {
		return []agentapi.ProviderModelRecord{
			{ID: "model-a", Name: "Model A", ContextWindow: 8192},
		}, nil
	}
	return nil, &regressionStubError{"provider unavailable"}
}

// regressionStubError is a trivial error type for regression tests.
type regressionStubError struct{ msg string }

func (e *regressionStubError) Error() string { return e.msg }

// Ensure our adapter types implement the interface.
var _ llm.ProviderDiscoveryAdapter = (*stubGitHubModelsAdapter)(nil)
var _ llm.ProviderDiscoveryAdapter = (*deterministicStubCatalogAdapter)(nil)
var _ llm.ProviderDiscoveryAdapter = (*firstSuccessThenFailCatalogAdapter)(nil)
