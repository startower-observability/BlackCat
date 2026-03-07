package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/startower-observability/blackcat/internal/agentapi"
)

// GeminiAdapter discovers models from the Gemini REST API.
type GeminiAdapter struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewGeminiAdapter creates a discovery adapter for Google Gemini.
// baseURL should be the API root (e.g. "https://generativelanguage.googleapis.com/v1beta").
func NewGeminiAdapter(apiKey, baseURL string) *GeminiAdapter {
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	return &GeminiAdapter{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// geminiListModelsResponse is the Gemini list models response.
type geminiListModelsResponse struct {
	Models []geminiModelEntry `json:"models"`
}

type geminiModelEntry struct {
	Name                       string   `json:"name"`                       // e.g. "models/gemini-2.5-pro"
	DisplayName                string   `json:"displayName"`                // e.g. "Gemini 2.5 Pro"
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"` // e.g. ["generateContent", "countTokens"]
	InputTokenLimit            int      `json:"inputTokenLimit"`
	OutputTokenLimit           int      `json:"outputTokenLimit"`
}

// DiscoverModels calls GET {baseURL}/models and normalizes to ProviderModelRecord.
// ContextWindow and MaxOutput are populated from the API response when available.
func (a *GeminiAdapter) DiscoverModels(ctx context.Context) ([]agentapi.ProviderModelRecord, error) {
	url := a.baseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("gemini discovery: create request: %w", err)
	}
	req.Header.Set("x-goog-api-key", a.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini discovery: request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini discovery: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini discovery: HTTP %d: %s", resp.StatusCode, truncateBody(body))
	}

	var modelsResp geminiListModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("gemini discovery: decode response: %w", err)
	}

	records := make([]agentapi.ProviderModelRecord, 0, len(modelsResp.Models))
	for _, m := range modelsResp.Models {
		// Extract model ID from "models/gemini-2.5-pro" → "gemini-2.5-pro"
		id := m.Name
		if strings.HasPrefix(id, "models/") {
			id = strings.TrimPrefix(id, "models/")
		}

		name := m.DisplayName
		if name == "" {
			name = id
		}

		// Gemini API actually returns token limits, so we use them (not fabricated).
		rec := agentapi.ProviderModelRecord{
			ID:            id,
			Name:          name,
			ContextWindow: m.InputTokenLimit,
			MaxOutput:     m.OutputTokenLimit,
			Freshness: agentapi.FreshnessMetadata{
				Source: agentapi.SourceLive,
			},
			// Modalities: zero — Gemini list endpoint doesn't reliably report these
		}
		records = append(records, rec)
	}
	return records, nil
}

// ProviderName returns "gemini".
func (a *GeminiAdapter) ProviderName() string {
	return "gemini"
}
