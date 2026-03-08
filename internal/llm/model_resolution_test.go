package llm

import (
	"errors"
	"testing"

	"github.com/startower-observability/blackcat/internal/config"
)

func TestResolveModelTargetAmbiguous(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.OpenAI.Enabled = true
	cfg.Providers.Copilot.Enabled = true
	cfg.Providers.Gemini.Enabled = true
	cfg.Providers.Zen.Enabled = true

	tests := []struct {
		name        string
		requestedID string
		backend     string
		configField string
		source      string
		raw         string
		canonical   string
		compat      []string
		env         []string
	}{
		{
			name:        "openai canonical resolves to openai backend",
			requestedID: "openai/gpt-4.1",
			backend:     "openai",
			configField: "providers.openai.model",
			source:      "explicit-provider",
			raw:         "gpt-4.1",
			canonical:   "openai/gpt-4.1",
			compat:      []string{"openai", "copilot"},
			env:         []string{"BLACKCAT_PROVIDERS_OPENAI_MODEL"},
		},
		{
			name:        "anthropic canonical resolves to zen backend",
			requestedID: "anthropic/claude-opus-4-6",
			backend:     "zen",
			configField: "providers.zen.model",
			source:      "explicit-provider",
			raw:         "claude-opus-4-6",
			canonical:   "anthropic/claude-opus-4-6",
			compat:      []string{"zen"},
			env:         []string{"BLACKCAT_PROVIDERS_ZEN_MODEL"},
		},
		{
			name:        "google canonical resolves to gemini backend",
			requestedID: "google/gemini-2.5-pro",
			backend:     "gemini",
			configField: "providers.gemini.model",
			source:      "explicit-provider",
			raw:         "gemini-2.5-pro",
			canonical:   "google/gemini-2.5-pro",
			compat:      []string{"gemini"},
			env:         []string{"BLACKCAT_PROVIDERS_GEMINI_MODEL"},
		},
		{
			name:        "codex maps to copilot",
			requestedID: "gpt-5.3-codex",
			backend:     "copilot",
			configField: "providers.copilot.model",
			source:      "explicit-provider",
			raw:         "gpt-5.3-codex",
			canonical:   "openai-codex/gpt-5.3-codex",
			compat:      []string{"copilot"},
			env:         []string{"BLACKCAT_PROVIDERS_COPILOT_MODEL"},
		},
		{
			name:        "unknown degrades gracefully to openai fallback",
			requestedID: "my-custom-model",
			backend:     "openai",
			configField: "providers.openai.model",
			source:      "fallback",
			raw:         "my-custom-model",
			canonical:   "unknown/my-custom-model",
			compat:      []string{"openai"},
			env:         []string{"BLACKCAT_PROVIDERS_OPENAI_MODEL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveModelTarget(cfg, tt.requestedID)
			if err != nil {
				t.Fatalf("ResolveModelTarget returned error: %v", err)
			}

			if got.BackendProvider != tt.backend {
				t.Errorf("backend = %q; want %q", got.BackendProvider, tt.backend)
			}
			if got.ConfigField != tt.configField {
				t.Errorf("configField = %q; want %q", got.ConfigField, tt.configField)
			}
			if got.Source != tt.source {
				t.Errorf("source = %q; want %q", got.Source, tt.source)
			}
			if got.RawModel != tt.raw {
				t.Errorf("rawModel = %q; want %q", got.RawModel, tt.raw)
			}
			if got.CanonicalID != tt.canonical {
				t.Errorf("canonicalID = %q; want %q", got.CanonicalID, tt.canonical)
			}
			if !equalStringSlice(got.CompatibleBackends, tt.compat) {
				t.Errorf("compatibleBackends = %#v; want %#v", got.CompatibleBackends, tt.compat)
			}
			if !equalStringSlice(got.EnvOverrideFields, tt.env) {
				t.Errorf("envOverrideFields = %#v; want %#v", got.EnvOverrideFields, tt.env)
			}
		})
	}
}

func TestResolveModelTargetNilConfig(t *testing.T) {
	_, err := ResolveModelTarget(nil, "gpt-4.1")
	if !errors.Is(err, ErrModelNotResolvable) {
		t.Fatalf("error = %v; want ErrModelNotResolvable", err)
	}
}

func TestResolveModelTargetVendorInferredWhenTargetBackendDisabled(t *testing.T) {
	cfg := &config.Config{}
	// keep openai disabled

	got, err := ResolveModelTarget(cfg, "openai/gpt-4.1")
	if err != nil {
		t.Fatalf("ResolveModelTarget returned error: %v", err)
	}

	if got.Source != "vendor-inferred" {
		t.Fatalf("source = %q; want %q", got.Source, "vendor-inferred")
	}
	if got.BackendProvider != "openai" {
		t.Fatalf("backend = %q; want %q", got.BackendProvider, "openai")
	}
	if !equalStringSlice(got.CompatibleBackends, []string{"openai", "copilot"}) {
		t.Fatalf("compatibleBackends = %#v; want %v", got.CompatibleBackends, []string{"openai", "copilot"})
	}
}

func TestResolveModelTargetOpenAIVendorReroutesToCopilotWhenOpenAIDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.Copilot.Enabled = true
	cfg.Providers.OpenAI.Enabled = false

	got, err := ResolveModelTarget(cfg, "gpt-5.2")
	if err != nil {
		t.Fatalf("ResolveModelTarget returned error: %v", err)
	}

	if got.BackendProvider != "copilot" {
		t.Fatalf("backend = %q; want %q", got.BackendProvider, "copilot")
	}
	if got.ConfigField != "providers.copilot.model" {
		t.Fatalf("configField = %q; want %q", got.ConfigField, "providers.copilot.model")
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestResolveModelTargetOpenAI(t *testing.T) {
	cfg := &config.Config{}

	got, err := ResolveModelTarget(cfg, "gpt-4.1")
	if err != nil {
		t.Fatalf("ResolveModelTarget returned error: %v", err)
	}

	if got.BackendProvider != "openai" {
		t.Fatalf("backend = %q; want %q", got.BackendProvider, "openai")
	}
	if got.ConfigField != "providers.openai.model" {
		t.Fatalf("configField = %q; want %q", got.ConfigField, "providers.openai.model")
	}
}

func TestResolveModelTargetCodex(t *testing.T) {
	cfg := &config.Config{}

	got, err := ResolveModelTarget(cfg, "gpt-5.3-codex")
	if err != nil {
		t.Fatalf("ResolveModelTarget returned error: %v", err)
	}

	if got.BackendProvider != "copilot" {
		t.Fatalf("backend = %q; want %q", got.BackendProvider, "copilot")
	}
	if got.ConfigField != "providers.copilot.model" {
		t.Fatalf("configField = %q; want %q", got.ConfigField, "providers.copilot.model")
	}
}

func TestResolveModelTargetFallback(t *testing.T) {
	cfg := &config.Config{}

	got, err := ResolveModelTarget(cfg, "unknownmodel-xyz")
	if err != nil {
		t.Fatalf("ResolveModelTarget returned error: %v", err)
	}

	if got.Source != "fallback" {
		t.Fatalf("source = %q; want %q", got.Source, "fallback")
	}
}
