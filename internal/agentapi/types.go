package agentapi

import "time"

// SourceLabel indicates the freshness/origin of a piece of data
type SourceLabel string

const (
	SourceLive        SourceLabel = "live"
	SourceCachedStale SourceLabel = "cached_stale"
	SourceStatic      SourceLabel = "static"
	SourceUnknown     SourceLabel = "unknown"
)

// FreshnessMetadata describes when and how data was last retrieved
type FreshnessMetadata struct {
	Source           SourceLabel `json:"source"`
	LastAttemptAt    time.Time   `json:"last_attempt_at,omitempty"`
	LastAttemptError string      `json:"last_attempt_error,omitempty"`
}

// ProviderModelRecord is the internal-rich representation of a model (may hold sensitive metadata)
type ProviderModelRecord struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Aliases       []string          `json:"aliases,omitempty"`
	ContextWindow int               `json:"context_window,omitempty"`
	MaxOutput     int               `json:"max_output,omitempty"`
	Modalities    []string          `json:"modalities,omitempty"`
	Freshness     FreshnessMetadata `json:"freshness"`
}

// ProviderModelView is the LLM-safe/redacted view (no pricing, no rate-limit tiers, no raw env var names)
type ProviderModelView struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Aliases       []string          `json:"aliases,omitempty"`
	ContextWindow int               `json:"context_window,omitempty"`
	Modalities    []string          `json:"modalities,omitempty"`
	Freshness     FreshnessMetadata `json:"freshness"`
}

// CatalogEntry holds a provider's known model list with freshness info
type CatalogEntry struct {
	Provider  string              `json:"provider"`
	Models    []ProviderModelView `json:"models,omitempty"`
	Freshness FreshnessMetadata   `json:"freshness"`
}

// RoleView is the LLM-safe view of a configured role
type RoleView struct {
	Name         string `json:"name"`
	Priority     int    `json:"priority"`
	KeywordCount int    `json:"keyword_count"`
}
