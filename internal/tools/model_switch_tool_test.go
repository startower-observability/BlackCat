package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/startower-observability/blackcat/internal/config"
)

func TestModelSwitchRequiresModel(t *testing.T) {
	tool := NewModelSwitchTool(&config.Config{})

	_, err := tool.Execute(context.Background(), mustJSON(map[string]string{"model": "   "}))
	if err == nil {
		t.Fatal("expected error for empty model")
	}
	if !strings.Contains(err.Error(), "model is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestModelSwitchResolvesCanonical(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "blackcat.yaml")

	input := `providers:
  openai:
    enabled: true
    model: gpt-4.1
  copilot:
    enabled: false
    model: gpt-4o
  gemini:
    enabled: false
    model: gemini-2.5-pro
  zen:
    enabled: true
    model: claude-3-5-sonnet
`
	if err := os.WriteFile(configPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input config: %v", err)
	}

	// Ensure shared config update mechanism has the active config path.
	_ = NewConfigUpdateTool(configPath)

	cfg := &config.Config{}
	cfg.Providers.Zen.Enabled = true

	tool := NewModelSwitchTool(cfg)
	result, err := tool.Execute(context.Background(), mustJSON(map[string]string{"model": "claude-opus-4-6"}))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if !strings.Contains(result, "anthropic/claude-opus-4-6") {
		t.Fatalf("expected canonical model in result, got: %q", result)
	}
	if !strings.Contains(result, "providers.zen.model") {
		t.Fatalf("expected resolved config field in result, got: %q", result)
	}

	updated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read updated config: %v", err)
	}
	if !strings.Contains(string(updated), "model: claude-opus-4-6") {
		t.Fatalf("expected zen model update, got:\n%s", string(updated))
	}
}

func TestModelSwitchRegistered(t *testing.T) {
	tool := NewModelSwitchTool(&config.Config{})

	if got := tool.Name(); got != "model_switch" {
		t.Fatalf("Name() = %q, want %q", got, "model_switch")
	}

	reg := NewRegistry()
	reg.Register(tool)

	found, err := reg.Get("model_switch")
	if err != nil {
		t.Fatalf("registry.Get(model_switch) returned error: %v", err)
	}
	if found.Name() != "model_switch" {
		t.Fatalf("registry returned tool with wrong name: %q", found.Name())
	}
}

func TestModelSwitchEmptyModelParam(t *testing.T) {
	tool := NewModelSwitchTool(&config.Config{})

	_, err := tool.Execute(context.Background(), mustJSON(map[string]string{"model": ""}))
	if err == nil {
		t.Fatal("expected error for empty model param")
	}
	if !strings.Contains(err.Error(), "model is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
