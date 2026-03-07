// Package discovery provides ProviderDiscoveryAdapter implementations for
// live and static model listing across BlackCat's supported LLM providers.
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

// OpenAIAdapter discovers models from the OpenAI /v1/models endpoint.
type OpenAIAdapter struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewOpenAIAdapter creates a discovery adapter for OpenAI-compatible endpoints.
// baseURL should be the API root (e.g. "https://api.openai.com/v1").
func NewOpenAIAdapter(apiKey, baseURL string) *OpenAIAdapter {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIAdapter{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// openaiModelsResponse is the response shape of GET /v1/models.
type openaiModelsResponse struct {
	Data []openaiModelEntry `json:"data"`
}

type openaiModelEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// DiscoverModels calls GET {baseURL}/models and normalizes to ProviderModelRecord.
// Unknown fields (ContextWindow, MaxOutput, Modalities) are left as zero values.
func (a *OpenAIAdapter) DiscoverModels(ctx context.Context) ([]agentapi.ProviderModelRecord, error) {
	url := a.baseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("openai discovery: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai discovery: request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai discovery: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai discovery: HTTP %d: %s", resp.StatusCode, truncateBody(body))
	}

	var modelsResp openaiModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("openai discovery: decode response: %w", err)
	}

	records := make([]agentapi.ProviderModelRecord, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		records = append(records, agentapi.ProviderModelRecord{
			ID:   m.ID,
			Name: m.ID, // OpenAI uses ID as display name
			Freshness: agentapi.FreshnessMetadata{
				Source: agentapi.SourceLive,
			},
			// ContextWindow, MaxOutput, Modalities: zero — do NOT fabricate
		})
	}
	return records, nil
}

// ProviderName returns "openai".
func (a *OpenAIAdapter) ProviderName() string {
	return "openai"
}
