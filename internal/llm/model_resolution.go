package llm

import (
	"errors"

	"github.com/startower-observability/blackcat/internal/config"
)

// ResolvedModelTarget is the result of resolving a canonical model ID to a specific
// backend configuration. It provides everything needed to switch the active model.
type ResolvedModelTarget struct {
	// BackendProvider is the BlackCat internal backend that handles this model
	// (e.g. "openai", "copilot", "gemini", "zen")
	BackendProvider string

	// ConfigField is the dot-separated YAML path to set in blackcat.yaml
	// (e.g. "providers.openai.model", "providers.copilot.model")
	ConfigField string

	// RawModel is the model ID the backend expects (no vendor prefix)
	RawModel string

	// CanonicalID is the normalized "vendor/model-name" form
	CanonicalID string

	// DisplayName is a human-friendly label
	DisplayName string

	// Source describes how this resolution was determined
	// (e.g. "explicit-provider", "vendor-inferred", "fallback")
	Source string

	// EnvOverrideFields lists environment variable names that override this model config
	// (e.g. ["BLACKCAT_PROVIDERS_OPENAI_MODEL"])
	EnvOverrideFields []string

	// CompatibleBackends lists all backend keys that could theoretically serve this model
	CompatibleBackends []string
}

// ErrModelNotResolvable is returned when the canonical ID cannot be mapped to any
// configured backend in cfg.
var ErrModelNotResolvable = errors.New("model not resolvable to any configured backend")

// ResolveModelTarget maps a canonical model ID (e.g. "anthropic/claude-opus-4-6") or
// raw model ID (e.g. "gpt-4.1") to a ResolvedModelTarget.
//
// Resolution order:
//  1. Canonicalize the input ID via CanonicalizeModelID
//  2. Match vendor to backend: anthropic→zen, openai→openai or copilot, openai-codex→copilot,
//     google→gemini, xai→zen, meta→zen, unknown→openai (fallback)
//  3. Populate ConfigField based on backend: "providers.{backend}.model"
//  4. Populate EnvOverrideFields based on backend
//  5. Check cfg to see if the target backend is enabled; set Source accordingly
//  6. CompatibleBackends: list all enabled backends that COULD serve this vendor family
func ResolveModelTarget(cfg *config.Config, requestedID string) (ResolvedModelTarget, error) {
	if cfg == nil {
		return ResolvedModelTarget{}, ErrModelNotResolvable
	}

	canonical := CanonicalizeModelID(requestedID)

	backend, cfgField := backendForVendor(canonical.Vendor)
	if canonical.Vendor == "" {
		backend = ""
		cfgField = ""
	}

	source := "vendor-inferred"
	if canonical.Vendor == "unknown" || (canonical.Vendor == "" && canonical.CanonicalID == "") {
		source = "fallback"
	} else if isBackendEnabled(cfg, backend) {
		source = "explicit-provider"
	}

	compatible := compatibleBackendsForVendor(canonical.Vendor)

	return ResolvedModelTarget{
		BackendProvider:    backend,
		ConfigField:        cfgField,
		RawModel:           canonical.RawModel,
		CanonicalID:        canonical.CanonicalID,
		DisplayName:        canonical.DisplayName,
		Source:             source,
		EnvOverrideFields:  envFieldsForBackend(backend),
		CompatibleBackends: compatible,
	}, nil
}

func backendForVendor(vendor string) (backend string, configField string) {
	switch vendor {
	case "anthropic", "xai", "meta":
		return "zen", "providers.zen.model"
	case "openai":
		return "openai", "providers.openai.model"
	case "openai-codex":
		return "copilot", "providers.copilot.model"
	case "google":
		return "gemini", "providers.gemini.model"
	case "unknown":
		fallthrough
	default:
		return "openai", "providers.openai.model"
	}
}

func envFieldsForBackend(backend string) []string {
	switch backend {
	case "openai":
		return []string{"BLACKCAT_PROVIDERS_OPENAI_MODEL"}
	case "copilot":
		return []string{"BLACKCAT_PROVIDERS_COPILOT_MODEL"}
	case "gemini":
		return []string{"BLACKCAT_PROVIDERS_GEMINI_MODEL"}
	case "zen":
		return []string{"BLACKCAT_PROVIDERS_ZEN_MODEL"}
	default:
		return nil
	}
}

func compatibleBackendsForVendor(vendor string) []string {
	switch vendor {
	case "anthropic", "xai", "meta":
		return []string{"zen"}
	case "openai":
		return []string{"openai", "copilot"}
	case "openai-codex":
		return []string{"copilot"}
	case "google":
		return []string{"gemini"}
	case "unknown":
		fallthrough
	default:
		return []string{"openai"}
	}
}

func isBackendEnabled(cfg *config.Config, backend string) bool {
	switch backend {
	case "openai":
		return cfg.Providers.OpenAI.Enabled
	case "copilot":
		return cfg.Providers.Copilot.Enabled
	case "gemini":
		return cfg.Providers.Gemini.Enabled
	case "zen":
		return cfg.Providers.Zen.Enabled
	default:
		return false
	}
}
