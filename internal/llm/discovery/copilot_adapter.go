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

const (
	// copilotTokenEndpoint exchanges GitHub OAuth token for a Copilot API token.
	copilotTokenEndpoint = "https://api.github.com/copilot_internal/v2/token"

	// copilotModelsEndpoint lists available models through the Copilot API.
	copilotModelsEndpoint = "https://api.githubcopilot.com/models"

	// githubModelsURLPattern is used to detect GitHub Models inference endpoints.
	githubModelsURLPattern = "models.inference.ai.azure.com"
)

// CopilotAdapter discovers models from the GitHub Copilot models endpoint.
// It handles the two-token architecture (OAuth → Copilot API token).
// If a GitHub Models endpoint is detected, models are tagged with a
// "github-models" alias.
type CopilotAdapter struct {
	oauthToken    string
	tokenEndpoint string
	modelsURL     string
	chatEndpoint  string // used to detect GitHub Models
	client        *http.Client
}

// NewCopilotAdapter creates a discovery adapter for GitHub Copilot.
// oauthToken is the long-lived GitHub OAuth token.
// chatEndpoint is the configured chat endpoint — used to detect GitHub Models.
func NewCopilotAdapter(oauthToken, chatEndpoint string) *CopilotAdapter {
	return &CopilotAdapter{
		oauthToken:    oauthToken,
		tokenEndpoint: copilotTokenEndpoint,
		modelsURL:     copilotModelsEndpoint,
		chatEndpoint:  chatEndpoint,
		client:        &http.Client{Timeout: 15 * time.Second},
	}
}

// copilotTokenResponse is the Copilot token exchange response.
type copilotDiscoveryTokenResp struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// copilotModelsResponse is the response from GET /models.
type copilotModelsResponse struct {
	Data []copilotModelEntry `json:"data"`
}

type copilotModelEntry struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// DiscoverModels fetches the Copilot model list.
// If the chat endpoint contains the GitHub Models URL pattern, each model
// is tagged with an Aliases entry "github-models".
func (a *CopilotAdapter) DiscoverModels(ctx context.Context) ([]agentapi.ProviderModelRecord, error) {
	// Step 1: Exchange OAuth token for Copilot API token
	apiToken, err := a.exchangeToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("copilot discovery: token exchange: %w", err)
	}

	// Step 2: List models
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("copilot discovery: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "GitHubCopilotChat/0.37.5")
	req.Header.Set("Editor-Version", "vscode/1.109.2")
	req.Header.Set("Copilot-Integration-Id", "vscode-chat")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("copilot discovery: request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("copilot discovery: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("copilot discovery: HTTP %d: %s", resp.StatusCode, truncateBody(body))
	}

	var modelsResp copilotModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("copilot discovery: decode response: %w", err)
	}

	isGitHubModels := strings.Contains(a.chatEndpoint, githubModelsURLPattern)

	records := make([]agentapi.ProviderModelRecord, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		name := m.Name
		if name == "" {
			name = m.ID
		}

		rec := agentapi.ProviderModelRecord{
			ID:   m.ID,
			Name: name,
			Freshness: agentapi.FreshnessMetadata{
				Source: agentapi.SourceLive,
			},
			// ContextWindow, MaxOutput, Modalities: zero — do NOT fabricate
		}

		if isGitHubModels {
			rec.Aliases = []string{"github-models"}
		}

		records = append(records, rec)
	}
	return records, nil
}

// ProviderName returns "copilot".
func (a *CopilotAdapter) ProviderName() string {
	return "copilot"
}

// exchangeToken exchanges the OAuth token for a short-lived Copilot API token.
func (a *CopilotAdapter) exchangeToken(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.tokenEndpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Authorization", "token "+a.oauthToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "GitHubCopilotChat/0.37.5")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token exchange HTTP %d: %s", resp.StatusCode, truncateBody(body))
	}

	var tokenResp copilotDiscoveryTokenResp
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	if tokenResp.Token == "" {
		return "", fmt.Errorf("empty token in response")
	}

	return tokenResp.Token, nil
}
