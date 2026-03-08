package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/startower-observability/blackcat/internal/config"
	"github.com/startower-observability/blackcat/internal/llm"
	"github.com/startower-observability/blackcat/internal/types"
)

const (
	modelSwitchToolName        = "model_switch"
	modelSwitchToolDescription = "Switch the active model using a canonical model ID (e.g. anthropic/claude-opus-4-6, openai/gpt-4.1) or raw model ID (e.g. claude-opus-4-6). Persists providers.{backend}.model in config; restart the daemon for changes to take effect."
)

var modelSwitchToolParameters = json.RawMessage(`{
	"type": "object",
	"properties": {
		"model": {
			"type": "string",
			"description": "Model ID to switch to, canonical (vendor/model) or raw model name"
		}
	},
	"required": ["model"]
}`)

// ModelSwitchTool resolves and persists active model selection.
type ModelSwitchTool struct {
	cfg *config.Config
}

var _ types.Tool = (*ModelSwitchTool)(nil)

func NewModelSwitchTool(cfg *config.Config) *ModelSwitchTool {
	return &ModelSwitchTool{cfg: cfg}
}

func (t *ModelSwitchTool) Name() string                { return modelSwitchToolName }
func (t *ModelSwitchTool) Description() string         { return modelSwitchToolDescription }
func (t *ModelSwitchTool) Parameters() json.RawMessage { return modelSwitchToolParameters }

func (t *ModelSwitchTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("model_switch: invalid arguments: %w", err)
	}

	requested := strings.TrimSpace(params.Model)
	if requested == "" {
		return "", fmt.Errorf("model_switch: model is required")
	}

	resolved, err := llm.ResolveModelTarget(t.cfg, requested)
	if err != nil {
		return "", fmt.Errorf("model_switch: resolve model target: %w", err)
	}
	if resolved.ConfigField == "" || strings.TrimSpace(resolved.RawModel) == "" {
		return "", fmt.Errorf("model_switch: resolved target is incomplete for model %q", requested)
	}

	if _, err := updateConfigField(getDefaultConfigPath(), resolved.ConfigField, resolved.RawModel); err != nil {
		return "", fmt.Errorf("model_switch: persist model: %w", err)
	}

	return fmt.Sprintf("Model switched to %s. Updated %s = %s. Restart daemon required for changes to take effect.", resolved.CanonicalID, resolved.ConfigField, resolved.RawModel), nil
}
