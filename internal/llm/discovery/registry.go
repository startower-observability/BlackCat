package discovery

import (
	"github.com/startower-observability/blackcat/internal/llm"
)

// AdapterConfig holds the credentials/endpoints needed to construct adapters.
type AdapterConfig struct {
	// OpenAI
	OpenAIKey     string
	OpenAIBaseURL string

	// Copilot
	CopilotOAuthToken string
	CopilotChatURL    string // used to detect GitHub Models

	// Gemini
	GeminiKey     string
	GeminiBaseURL string
}

// RegisterDefaultAdapters creates and registers discovery adapters for all
// known providers on the given ProviderCatalogCache. Adapters are only
// registered if their required credentials are present.
//
// Live adapters: OpenAI, Copilot, Gemini
// Static adapters: Antigravity, Zen (always registered)
func RegisterDefaultAdapters(cache *llm.ProviderCatalogCache, cfg AdapterConfig) {
	// Live: OpenAI
	if cfg.OpenAIKey != "" {
		cache.RegisterAdapter(NewOpenAIAdapter(cfg.OpenAIKey, cfg.OpenAIBaseURL))
	}

	// Live: Copilot (+ GitHub Models alias detection)
	if cfg.CopilotOAuthToken != "" {
		cache.RegisterAdapter(NewCopilotAdapter(cfg.CopilotOAuthToken, cfg.CopilotChatURL))
	}

	// Live: Gemini
	if cfg.GeminiKey != "" {
		cache.RegisterAdapter(NewGeminiAdapter(cfg.GeminiKey, cfg.GeminiBaseURL))
	}

	// Static: Antigravity (no standard list endpoint)
	cache.RegisterAdapter(NewAntigravityStaticAdapter())

	// Static: Zen (curated model list)
	cache.RegisterAdapter(NewZenStaticAdapter())
}
