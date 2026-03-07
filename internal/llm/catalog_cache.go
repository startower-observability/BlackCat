package llm

import (
	"context"
	"sync"
	"time"

	"github.com/startower-observability/blackcat/internal/agentapi"
)

// ProviderDiscoveryAdapter is the interface a provider backend must implement
// to support live model discovery. Not all backends need to implement this.
type ProviderDiscoveryAdapter interface {
	// DiscoverModels fetches the current model list for this provider.
	// Returns a fresh []ProviderModelRecord or error.
	DiscoverModels(ctx context.Context) ([]agentapi.ProviderModelRecord, error)
	// ProviderName returns the canonical provider name (e.g. "openai", "copilot").
	ProviderName() string
}

// catalogEntry holds cached data for one provider.
type catalogEntry struct {
	models    []agentapi.ProviderModelRecord
	fetchedAt time.Time
	lastError string
}

// ProviderCatalogCache is a thread-safe, on-demand cache for provider model lists.
// It supports stale fallback: if a refresh fails, it returns the old data with
// source=SourceCachedStale and the error message attached.
type ProviderCatalogCache struct {
	mu       sync.RWMutex
	ttl      time.Duration
	adapters map[string]ProviderDiscoveryAdapter // keyed by provider name
	cache    map[string]*catalogEntry
}

// NewProviderCatalogCache creates a cache with the given TTL.
func NewProviderCatalogCache(ttl time.Duration) *ProviderCatalogCache {
	return &ProviderCatalogCache{
		ttl:      ttl,
		adapters: make(map[string]ProviderDiscoveryAdapter),
		cache:    make(map[string]*catalogEntry),
	}
}

// RegisterAdapter registers a discovery adapter for a provider.
func (c *ProviderCatalogCache) RegisterAdapter(adapter ProviderDiscoveryAdapter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.adapters[adapter.ProviderName()] = adapter
}

// Get returns the current catalog for a provider.
// If cache is fresh (within TTL), returns cached data with SourceLive.
// If cache is expired, triggers a refresh. If refresh fails, returns stale with SourceCachedStale.
// If no adapter is registered for the provider, returns an empty CatalogEntry with SourceUnknown.
func (c *ProviderCatalogCache) Get(ctx context.Context, provider string) agentapi.CatalogEntry {
	c.mu.RLock()
	entry := c.cache[provider]
	adapter := c.adapters[provider]
	c.mu.RUnlock()

	now := time.Now()
	if entry != nil && now.Sub(entry.fetchedAt) < c.ttl {
		// Cache is fresh
		return buildCatalogEntry(provider, entry, agentapi.SourceLive)
	}

	// Need refresh
	if adapter == nil {
		if entry != nil {
			return buildCatalogEntry(provider, entry, agentapi.SourceCachedStale)
		}
		return agentapi.CatalogEntry{
			Provider:  provider,
			Freshness: agentapi.FreshnessMetadata{Source: agentapi.SourceUnknown},
		}
	}

	// Refresh via adapter
	models, err := adapter.DiscoverModels(ctx)

	c.mu.Lock()
	defer c.mu.Unlock()

	if err != nil {
		// On error, keep old data if available
		if entry == nil {
			entry = &catalogEntry{}
		}
		entry.lastError = err.Error()
		c.cache[provider] = entry
		return buildCatalogEntry(provider, entry, agentapi.SourceCachedStale)
	}

	newEntry := &catalogEntry{
		models:    models,
		fetchedAt: now,
		lastError: "",
	}
	c.cache[provider] = newEntry
	return buildCatalogEntry(provider, newEntry, agentapi.SourceLive)
}

// Refresh forces a cache refresh for the given provider, regardless of TTL.
// Returns the updated CatalogEntry.
func (c *ProviderCatalogCache) Refresh(ctx context.Context, provider string) agentapi.CatalogEntry {
	c.mu.RLock()
	adapter := c.adapters[provider]
	entry := c.cache[provider]
	c.mu.RUnlock()

	if adapter == nil {
		if entry != nil {
			return buildCatalogEntry(provider, entry, agentapi.SourceCachedStale)
		}
		return agentapi.CatalogEntry{
			Provider:  provider,
			Freshness: agentapi.FreshnessMetadata{Source: agentapi.SourceUnknown},
		}
	}

	now := time.Now()
	models, err := adapter.DiscoverModels(ctx)

	c.mu.Lock()
	defer c.mu.Unlock()

	if err != nil {
		if entry == nil {
			entry = &catalogEntry{}
		}
		entry.lastError = err.Error()
		c.cache[provider] = entry
		return buildCatalogEntry(provider, entry, agentapi.SourceCachedStale)
	}

	newEntry := &catalogEntry{
		models:    models,
		fetchedAt: now,
		lastError: "",
	}
	c.cache[provider] = newEntry
	return buildCatalogEntry(provider, newEntry, agentapi.SourceLive)
}

// Providers returns the list of provider names that have registered adapters.
func (c *ProviderCatalogCache) Providers() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	names := make([]string, 0, len(c.adapters))
	for k := range c.adapters {
		names = append(names, k)
	}
	return names
}

// buildCatalogEntry converts internal cache data to a CatalogEntry with ProviderModelView (redacted).
func buildCatalogEntry(provider string, entry *catalogEntry, source agentapi.SourceLabel) agentapi.CatalogEntry {
	views := make([]agentapi.ProviderModelView, 0, len(entry.models))
	for _, m := range entry.models {
		views = append(views, agentapi.ProviderModelView{
			ID:            m.ID,
			Name:          m.Name,
			Aliases:       m.Aliases,
			ContextWindow: m.ContextWindow,
			Modalities:    m.Modalities,
			Freshness:     agentapi.FreshnessMetadata{Source: source},
		})
	}
	return agentapi.CatalogEntry{
		Provider: provider,
		Models:   views,
		Freshness: agentapi.FreshnessMetadata{
			Source:           source,
			LastAttemptAt:    entry.fetchedAt,
			LastAttemptError: entry.lastError,
		},
	}
}
