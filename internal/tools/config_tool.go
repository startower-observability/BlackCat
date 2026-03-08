package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
	"github.com/startower-observability/blackcat/internal/config"
	"github.com/startower-observability/blackcat/internal/types"
)

const (
	configUpdateToolName        = "config_update"
	configUpdateToolDescription = "Update a non-protected YAML configuration field at runtime"
)

var configUpdateToolParameters = json.RawMessage(`{
	"type": "object",
	"properties": {
		"field": {
			"type": "string",
			"description": "YAML field path in dot notation, e.g. 'agent.name' or 'providers.openai.model'. LEGACY: 'llm.model' should not be used for model switching; use model_switch tool instead. For direct edits, use providers.{backend}.model."
		},
		"value": {
			"type": "string",
			"description": "New value to set for the field"
		}
	},
	"required": ["field", "value"]
}`)

// ConfigUpdateTool updates configurable YAML fields in-place.
type ConfigUpdateTool struct {
	configPath string // absolute path to blackcat.yaml
}

var (
	defaultConfigPathMu sync.RWMutex
	defaultConfigPath   string
)

var _ types.Tool = (*ConfigUpdateTool)(nil)

func NewConfigUpdateTool(configPath string) *ConfigUpdateTool {
	setDefaultConfigPath(configPath)
	return &ConfigUpdateTool{configPath: configPath}
}

func (t *ConfigUpdateTool) Name() string                { return configUpdateToolName }
func (t *ConfigUpdateTool) Description() string         { return configUpdateToolDescription }
func (t *ConfigUpdateTool) Parameters() json.RawMessage { return configUpdateToolParameters }

func (t *ConfigUpdateTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Field string `json:"field"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("config_update: invalid arguments: %w", err)
	}

	field := strings.TrimSpace(params.Field)
	if field == "" {
		return "", fmt.Errorf("config_update: field is required")
	}
	if strings.TrimSpace(params.Value) == "" {
		return "", fmt.Errorf("config_update: value is required")
	}

	if config.IsProtected(field) {
		return "", fmt.Errorf("config_update: %s", config.ProtectedReason(field))
	}

	return updateConfigField(t.configPath, field, params.Value)
}

func setDefaultConfigPath(configPath string) {
	defaultConfigPathMu.Lock()
	defer defaultConfigPathMu.Unlock()
	defaultConfigPath = strings.TrimSpace(configPath)
}

func getDefaultConfigPath() string {
	defaultConfigPathMu.RLock()
	defer defaultConfigPathMu.RUnlock()
	return defaultConfigPath
}

func updateConfigField(configPath, field, value string) (string, error) {
	if strings.TrimSpace(configPath) == "" {
		return "", fmt.Errorf("config_update: config path is required")
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("config_update: read config: %w", err)
	}

	astFile, err := parser.ParseBytes(content, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("config_update: parse yaml: %w", err)
	}

	path, err := yaml.PathString("$." + field)
	if err != nil {
		return "", fmt.Errorf("config_update: invalid field path: %w", err)
	}

	replacementValue := coerceStringValue(value)
	replacementYAML, err := yaml.Marshal(replacementValue)
	if err != nil {
		return "", fmt.Errorf("config_update: marshal value: %w", err)
	}

	if err := path.ReplaceWithReader(astFile, bytes.NewReader(replacementYAML)); err != nil {
		return "", fmt.Errorf("config_update: update field %q: %w", field, err)
	}

	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(configPath); statErr == nil {
		mode = info.Mode()
	}

	if err := os.WriteFile(configPath, []byte(astFile.String()), mode); err != nil {
		return "", fmt.Errorf("config_update: write config: %w", err)
	}

	return fmt.Sprintf("Config updated: %s = %s", field, value), nil
}

func coerceStringValue(raw string) interface{} {
	trimmed := strings.TrimSpace(raw)
	lower := strings.ToLower(trimmed)

	if lower == "true" {
		return true
	}
	if lower == "false" {
		return false
	}

	if i, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return f
	}

	return raw
}
