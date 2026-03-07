package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/startower-observability/blackcat/internal/agentapi"
)

// TestProviderModelDiscoveryNormalization verifies that each live adapter
// correctly normalizes the provider's model list response into
// []ProviderModelRecord with proper ID, Name, and Freshness fields.
func TestProviderModelDiscoveryNormalization(t *testing.T) {
	t.Run("OpenAI", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/models" {
				t.Errorf("unexpected path: %s", r.URL.Path)
				http.NotFound(w, r)
				return
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Errorf("expected auth header 'Bearer test-key', got %q", got)
			}
			resp := openaiModelsResponse{
				Data: []openaiModelEntry{
					{ID: "gpt-4o", Object: "model", OwnedBy: "openai"},
					{ID: "gpt-4o-mini", Object: "model", OwnedBy: "openai"},
					{ID: "o3", Object: "model", OwnedBy: "openai"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		adapter := NewOpenAIAdapter("test-key", srv.URL+"/v1")
		models, err := adapter.DiscoverModels(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(models) != 3 {
			t.Fatalf("expected 3 models, got %d", len(models))
		}

		// Verify normalization
		expected := []struct {
			id   string
			name string
		}{
			{"gpt-4o", "gpt-4o"},
			{"gpt-4o-mini", "gpt-4o-mini"},
			{"o3", "o3"},
		}
		for i, e := range expected {
			if models[i].ID != e.id {
				t.Errorf("model[%d].ID = %q, want %q", i, models[i].ID, e.id)
			}
			if models[i].Name != e.name {
				t.Errorf("model[%d].Name = %q, want %q", i, models[i].Name, e.name)
			}
			if models[i].Freshness.Source != agentapi.SourceLive {
				t.Errorf("model[%d].Freshness.Source = %q, want %q", i, models[i].Freshness.Source, agentapi.SourceLive)
			}
		}

		if adapter.ProviderName() != "openai" {
			t.Errorf("ProviderName() = %q, want %q", adapter.ProviderName(), "openai")
		}
	})

	t.Run("Copilot", func(t *testing.T) {
		// Mock both token exchange and model listing
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/token":
				resp := copilotDiscoveryTokenResp{Token: "cpt_abc123", ExpiresAt: 9999999999}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			case "/models":
				if got := r.Header.Get("Authorization"); got != "Bearer cpt_abc123" {
					t.Errorf("models request: expected auth 'Bearer cpt_abc123', got %q", got)
				}
				resp := copilotModelsResponse{
					Data: []copilotModelEntry{
						{ID: "gpt-4o", Name: "GPT-4o"},
						{ID: "claude-sonnet-4-6", Name: "Claude Sonnet 4.6"},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		adapter := NewCopilotAdapter("test-oauth-token", "https://api.githubcopilot.com/chat/completions")
		// Override endpoints to use test server
		adapter.tokenEndpoint = srv.URL + "/token"
		adapter.modelsURL = srv.URL + "/models"

		models, err := adapter.DiscoverModels(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(models) != 2 {
			t.Fatalf("expected 2 models, got %d", len(models))
		}
		if models[0].ID != "gpt-4o" || models[0].Name != "GPT-4o" {
			t.Errorf("model[0] = {%q, %q}, want {gpt-4o, GPT-4o}", models[0].ID, models[0].Name)
		}
		if models[1].ID != "claude-sonnet-4-6" {
			t.Errorf("model[1].ID = %q, want %q", models[1].ID, "claude-sonnet-4-6")
		}
		// Regular Copilot endpoint should not add github-models alias
		for _, m := range models {
			if len(m.Aliases) > 0 {
				t.Errorf("model %q should have no aliases (regular copilot), got %v", m.ID, m.Aliases)
			}
		}

		if adapter.ProviderName() != "copilot" {
			t.Errorf("ProviderName() = %q, want %q", adapter.ProviderName(), "copilot")
		}
	})

	t.Run("CopilotGitHubModels", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/token":
				resp := copilotDiscoveryTokenResp{Token: "cpt_xyz", ExpiresAt: 9999999999}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			case "/models":
				resp := copilotModelsResponse{
					Data: []copilotModelEntry{
						{ID: "gpt-4o", Name: "GPT-4o"},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		// GitHub Models endpoint triggers alias tagging
		adapter := NewCopilotAdapter("test-oauth-token", "https://models.inference.ai.azure.com/chat/completions")
		adapter.tokenEndpoint = srv.URL + "/token"
		adapter.modelsURL = srv.URL + "/models"

		models, err := adapter.DiscoverModels(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(models) != 1 {
			t.Fatalf("expected 1 model, got %d", len(models))
		}
		if len(models[0].Aliases) != 1 || models[0].Aliases[0] != "github-models" {
			t.Errorf("expected aliases [github-models], got %v", models[0].Aliases)
		}
	})

	t.Run("Gemini", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1beta/models" {
				t.Errorf("unexpected path: %s", r.URL.Path)
				http.NotFound(w, r)
				return
			}
			if got := r.Header.Get("x-goog-api-key"); got != "test-gemini-key" {
				t.Errorf("expected x-goog-api-key 'test-gemini-key', got %q", got)
			}
			resp := geminiListModelsResponse{
				Models: []geminiModelEntry{
					{
						Name:                       "models/gemini-2.5-pro",
						DisplayName:                "Gemini 2.5 Pro",
						InputTokenLimit:            1048576,
						OutputTokenLimit:           65536,
						SupportedGenerationMethods: []string{"generateContent", "countTokens"},
					},
					{
						Name:             "models/gemini-2.5-flash",
						DisplayName:      "Gemini 2.5 Flash",
						InputTokenLimit:  1048576,
						OutputTokenLimit: 65536,
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		adapter := NewGeminiAdapter("test-gemini-key", srv.URL+"/v1beta")
		models, err := adapter.DiscoverModels(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(models) != 2 {
			t.Fatalf("expected 2 models, got %d", len(models))
		}

		// Verify "models/" prefix is stripped
		if models[0].ID != "gemini-2.5-pro" {
			t.Errorf("model[0].ID = %q, want %q", models[0].ID, "gemini-2.5-pro")
		}
		if models[0].Name != "Gemini 2.5 Pro" {
			t.Errorf("model[0].Name = %q, want %q", models[0].Name, "Gemini 2.5 Pro")
		}
		// Gemini provides actual token limits — should be preserved
		if models[0].ContextWindow != 1048576 {
			t.Errorf("model[0].ContextWindow = %d, want 1048576", models[0].ContextWindow)
		}
		if models[0].MaxOutput != 65536 {
			t.Errorf("model[0].MaxOutput = %d, want 65536", models[0].MaxOutput)
		}
		if models[0].Freshness.Source != agentapi.SourceLive {
			t.Errorf("model[0].Freshness.Source = %q, want %q", models[0].Freshness.Source, agentapi.SourceLive)
		}

		if adapter.ProviderName() != "gemini" {
			t.Errorf("ProviderName() = %q, want %q", adapter.ProviderName(), "gemini")
		}
	})

	t.Run("StaticZen", func(t *testing.T) {
		adapter := NewZenStaticAdapter()
		models, err := adapter.DiscoverModels(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(models) != 3 {
			t.Fatalf("expected 3 models, got %d", len(models))
		}
		if models[0].ID != "opencode/claude-opus-4-6" {
			t.Errorf("model[0].ID = %q, want %q", models[0].ID, "opencode/claude-opus-4-6")
		}
		for _, m := range models {
			if m.Freshness.Source != agentapi.SourceStatic {
				t.Errorf("model %q Freshness.Source = %q, want %q", m.ID, m.Freshness.Source, agentapi.SourceStatic)
			}
		}
		if adapter.ProviderName() != "zen" {
			t.Errorf("ProviderName() = %q, want %q", adapter.ProviderName(), "zen")
		}
	})

	t.Run("StaticAntigravity", func(t *testing.T) {
		adapter := NewAntigravityStaticAdapter()
		models, err := adapter.DiscoverModels(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(models) != 0 {
			t.Fatalf("expected 0 models (empty static), got %d", len(models))
		}
		if adapter.ProviderName() != "antigravity" {
			t.Errorf("ProviderName() = %q, want %q", adapter.ProviderName(), "antigravity")
		}
	})
}

// TestProviderModelDiscoveryMarksUnknownFields verifies that adapters do NOT
// fabricate values for fields they cannot determine from the API response.
// OpenAI: ContextWindow=0, MaxOutput=0, Modalities=nil
// Gemini: ContextWindow and MaxOutput from API, Modalities=nil
// Copilot: ContextWindow=0, MaxOutput=0, Modalities=nil
func TestProviderModelDiscoveryMarksUnknownFields(t *testing.T) {
	t.Run("OpenAI_UnknownFields", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := openaiModelsResponse{
				Data: []openaiModelEntry{
					{ID: "gpt-5.2", Object: "model", OwnedBy: "openai"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		adapter := NewOpenAIAdapter("key", srv.URL+"/v1")
		models, err := adapter.DiscoverModels(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(models) != 1 {
			t.Fatalf("expected 1 model, got %d", len(models))
		}

		m := models[0]
		if m.ContextWindow != 0 {
			t.Errorf("ContextWindow = %d, want 0 (unknown, not fabricated)", m.ContextWindow)
		}
		if m.MaxOutput != 0 {
			t.Errorf("MaxOutput = %d, want 0 (unknown, not fabricated)", m.MaxOutput)
		}
		if m.Modalities != nil {
			t.Errorf("Modalities = %v, want nil (unknown, not fabricated)", m.Modalities)
		}
		if len(m.Aliases) != 0 {
			t.Errorf("Aliases = %v, want empty (no aliases for OpenAI)", m.Aliases)
		}
	})

	t.Run("Copilot_UnknownFields", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/token":
				json.NewEncoder(w).Encode(copilotDiscoveryTokenResp{Token: "tok", ExpiresAt: 9999999999})
			case "/models":
				resp := copilotModelsResponse{
					Data: []copilotModelEntry{
						{ID: "claude-sonnet-4-6", Name: "Claude Sonnet 4.6"},
					},
				}
				json.NewEncoder(w).Encode(resp)
			}
		}))
		defer srv.Close()

		adapter := NewCopilotAdapter("oauth", "https://api.githubcopilot.com/chat/completions")
		adapter.tokenEndpoint = srv.URL + "/token"
		adapter.modelsURL = srv.URL + "/models"

		models, err := adapter.DiscoverModels(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		m := models[0]
		if m.ContextWindow != 0 {
			t.Errorf("ContextWindow = %d, want 0 (unknown)", m.ContextWindow)
		}
		if m.MaxOutput != 0 {
			t.Errorf("MaxOutput = %d, want 0 (unknown)", m.MaxOutput)
		}
		if m.Modalities != nil {
			t.Errorf("Modalities = %v, want nil (unknown)", m.Modalities)
		}
	})

	t.Run("Gemini_KnownFieldsFromAPI", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := geminiListModelsResponse{
				Models: []geminiModelEntry{
					{
						Name:             "models/gemini-2.5-flash",
						DisplayName:      "Gemini 2.5 Flash",
						InputTokenLimit:  1048576,
						OutputTokenLimit: 65536,
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		adapter := NewGeminiAdapter("key", srv.URL+"/v1beta")
		models, err := adapter.DiscoverModels(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		m := models[0]
		// Gemini returns real values — not fabricated
		if m.ContextWindow != 1048576 {
			t.Errorf("ContextWindow = %d, want 1048576 (from API)", m.ContextWindow)
		}
		if m.MaxOutput != 65536 {
			t.Errorf("MaxOutput = %d, want 65536 (from API)", m.MaxOutput)
		}
		// Modalities still unknown from list endpoint
		if m.Modalities != nil {
			t.Errorf("Modalities = %v, want nil (unknown from list endpoint)", m.Modalities)
		}
	})

	t.Run("Gemini_ZeroTokenLimits", func(t *testing.T) {
		// When the API returns 0 for token limits, adapter should not fabricate
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := geminiListModelsResponse{
				Models: []geminiModelEntry{
					{
						Name:             "models/some-experimental",
						DisplayName:      "Experimental",
						InputTokenLimit:  0,
						OutputTokenLimit: 0,
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		adapter := NewGeminiAdapter("key", srv.URL+"/v1beta")
		models, err := adapter.DiscoverModels(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		m := models[0]
		if m.ContextWindow != 0 {
			t.Errorf("ContextWindow = %d, want 0", m.ContextWindow)
		}
		if m.MaxOutput != 0 {
			t.Errorf("MaxOutput = %d, want 0", m.MaxOutput)
		}
	})

	t.Run("StaticZen_UnknownFields", func(t *testing.T) {
		adapter := NewZenStaticAdapter()
		models, err := adapter.DiscoverModels(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, m := range models {
			if m.ContextWindow != 0 {
				t.Errorf("model %q ContextWindow = %d, want 0 (static, not fabricated)", m.ID, m.ContextWindow)
			}
			if m.MaxOutput != 0 {
				t.Errorf("model %q MaxOutput = %d, want 0 (static, not fabricated)", m.ID, m.MaxOutput)
			}
			if m.Modalities != nil {
				t.Errorf("model %q Modalities = %v, want nil (static, not fabricated)", m.ID, m.Modalities)
			}
		}
	})
}

// TestDiscoveryAdapterHTTPErrors verifies that adapters return errors on HTTP failures.
func TestDiscoveryAdapterHTTPErrors(t *testing.T) {
	t.Run("OpenAI_500", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "internal"}`))
		}))
		defer srv.Close()

		adapter := NewOpenAIAdapter("key", srv.URL+"/v1")
		_, err := adapter.DiscoverModels(context.Background())
		if err == nil {
			t.Fatal("expected error on 500 response")
		}
	})

	t.Run("Copilot_TokenExchangeFails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message": "Bad credentials"}`))
		}))
		defer srv.Close()

		adapter := NewCopilotAdapter("bad-token", "")
		adapter.tokenEndpoint = srv.URL + "/token"

		_, err := adapter.DiscoverModels(context.Background())
		if err == nil {
			t.Fatal("expected error on token exchange failure")
		}
	})

	t.Run("Gemini_403", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error": {"message": "API key invalid"}}`))
		}))
		defer srv.Close()

		adapter := NewGeminiAdapter("bad-key", srv.URL+"/v1beta")
		_, err := adapter.DiscoverModels(context.Background())
		if err == nil {
			t.Fatal("expected error on 403 response")
		}
	})
}
