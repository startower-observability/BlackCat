package discovery

import (
	"context"

	"github.com/startower-observability/blackcat/internal/agentapi"
)

// StaticAdapter is a ProviderDiscoveryAdapter that returns a fixed list of models.
// Used for providers that don't have a standard model listing endpoint
// (e.g. Antigravity, Zen).
type StaticAdapter struct {
	name   string
	models []agentapi.ProviderModelRecord
}

// NewStaticAdapter creates an adapter that always returns the given static model list.
func NewStaticAdapter(providerName string, models []agentapi.ProviderModelRecord) *StaticAdapter {
	return &StaticAdapter{
		name:   providerName,
		models: models,
	}
}

// DiscoverModels returns the static model list with SourceStatic freshness.
func (a *StaticAdapter) DiscoverModels(_ context.Context) ([]agentapi.ProviderModelRecord, error) {
	result := make([]agentapi.ProviderModelRecord, len(a.models))
	copy(result, a.models)
	for i := range result {
		result[i].Freshness = agentapi.FreshnessMetadata{
			Source: agentapi.SourceStatic,
		}
	}
	return result, nil
}

// ProviderName returns the provider name.
func (a *StaticAdapter) ProviderName() string {
	return a.name
}

// NewAntigravityStaticAdapter creates a static adapter for Antigravity.
// Antigravity uses an internal Google API without a standard model listing endpoint.
func NewAntigravityStaticAdapter() *StaticAdapter {
	return NewStaticAdapter("antigravity", []agentapi.ProviderModelRecord{})
}

// NewZenStaticAdapter creates a static adapter for Zen Coding Plan
// using the known curated model list.
func NewZenStaticAdapter() *StaticAdapter {
	models := []agentapi.ProviderModelRecord{
		{ID: "opencode/claude-opus-4-6", Name: "Claude Opus 4.6 (Zen)"},
		{ID: "opencode/claude-sonnet-4-6", Name: "Claude Sonnet 4.6 (Zen)"},
		{ID: "opencode/gemini-3.1-pro", Name: "Gemini 3.1 Pro (Zen)"},
	}
	return NewStaticAdapter("zen", models)
}
