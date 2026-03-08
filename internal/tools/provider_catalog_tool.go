package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/startower-observability/blackcat/internal/agentapi"
	"github.com/startower-observability/blackcat/internal/llm"
)

const (
	providerCatalogToolName        = "provider_catalog"
	providerCatalogToolDescription = "Get the list of AI models available from a specific provider, or all providers. Use this when asked about available models, what models a provider supports, or what GitHub Copilot/OpenAI models are available."
)

var providerCatalogToolParameters = json.RawMessage(`{
	"type": "object",
	"properties": {
		"provider": {
			"type": "string",
			"description": "Provider name (e.g. 'openai', 'copilot', 'gemini'). Omit for all providers."
		}
	}
}`)

// ProviderCatalogTool lets the agent query which AI models are available.
type ProviderCatalogTool struct {
	cache *llm.ProviderCatalogCache
}

// NewProviderCatalogTool creates a ProviderCatalogTool.
func NewProviderCatalogTool(cache *llm.ProviderCatalogCache) *ProviderCatalogTool {
	return &ProviderCatalogTool{cache: cache}
}

func (t *ProviderCatalogTool) Name() string                { return providerCatalogToolName }
func (t *ProviderCatalogTool) Description() string         { return providerCatalogToolDescription }
func (t *ProviderCatalogTool) Parameters() json.RawMessage { return providerCatalogToolParameters }

// normalizeProvider maps alias names to canonical provider names.
func normalizeProvider(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "github-models", "github_models", "github":
		return "copilot"
	default:
		return n
	}
}

func (t *ProviderCatalogTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var args struct {
		Provider string `json:"provider"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &args)
	}

	if t.cache == nil {
		return "No catalog available. [source: unknown]", nil
	}

	if args.Provider != "" {
		// Single provider query
		provider := normalizeProvider(args.Provider)
		entry := t.cache.Get(ctx, provider)
		if len(entry.Models) == 0 && entry.Freshness.Source == agentapi.SourceUnknown {
			return fmt.Sprintf("No catalog available for provider: %s. [source: unknown]", provider), nil
		}
		return formatCatalogEntries([]agentapi.CatalogEntry{entry})
	}

	// All providers
	providers := t.cache.Providers()
	if len(providers) == 0 {
		return "No providers registered. [source: unknown]", nil
	}

	entries := make([]agentapi.CatalogEntry, 0, len(providers))
	for _, p := range providers {
		entries = append(entries, t.cache.Get(ctx, p))
	}
	return formatCatalogEntries(entries)
}

// formatCatalogEntries formats catalog entries as JSON for the LLM.
func formatCatalogEntries(entries []agentapi.CatalogEntry) (string, error) {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("provider_catalog: marshal failed: %w", err)
	}
	return string(data), nil
}
