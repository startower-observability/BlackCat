package tools

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/startower-observability/blackcat/internal/agentapi"
	"github.com/startower-observability/blackcat/internal/config"
	"github.com/startower-observability/blackcat/internal/llm"
	"github.com/startower-observability/blackcat/internal/service"
)

func TestIntegrationModelSwitchToClaudeUpdatesSelfStatus(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "blackcat.yaml")

	input := `providers:
  openai:
    enabled: true
    model: gpt-4.1
  copilot:
    enabled: true
    model: gpt-5.3-codex
  gemini:
    enabled: true
    model: gemini-2.5-pro
  zen:
    enabled: true
    model: claude-3-5-sonnet
`
	if err := os.WriteFile(configPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input config: %v", err)
	}

	// model_switch persists through the shared config update helper.
	_ = NewConfigUpdateTool(configPath)

	cfg := &config.Config{}
	cfg.Providers.OpenAI.Enabled = true
	cfg.Providers.Copilot.Enabled = true
	cfg.Providers.Gemini.Enabled = true
	cfg.Providers.Zen.Enabled = true

	holder := agentapi.NewRuntimeModelHolder()
	holder.Set(agentapi.RuntimeModelStatus{
		ConfiguredModel: agentapi.RuntimeModelRef{
			CanonicalID:     "openai/gpt-4.1",
			Vendor:          "openai",
			RawModel:        "gpt-4.1",
			BackendProvider: "openai",
		},
		AppliedModel: agentapi.RuntimeModelRef{
			CanonicalID:     "openai/gpt-4.1",
			Vendor:          "openai",
			RawModel:        "gpt-4.1",
			BackendProvider: "openai",
		},
		BackendProvider: "openai",
	})

	modelSwitch := NewModelSwitchTool(cfg)
	result, err := modelSwitch.Execute(context.Background(), mustJSON(map[string]string{"model": "claude-opus-4-6"}))
	if err != nil {
		t.Fatalf("model_switch returned error: %v", err)
	}
	if !strings.Contains(result, "anthropic/claude-opus-4-6") {
		t.Fatalf("expected canonical model in result, got: %q", result)
	}
	if !strings.Contains(result, "providers.zen.model") {
		t.Fatalf("expected zen config field in result, got: %q", result)
	}

	updatedConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read updated config: %v", err)
	}
	if !strings.Contains(string(updatedConfig), "model: claude-opus-4-6") {
		t.Fatalf("expected persisted zen model update, got:\n%s", string(updatedConfig))
	}

	// Simulate daemon watcher applying the resolved model into runtime holder.
	resolved, err := llm.ResolveModelTarget(cfg, "claude-opus-4-6")
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	holder.Set(agentapi.RuntimeModelStatus{
		ConfiguredModel: agentapi.RuntimeModelRef{
			CanonicalID:     resolved.CanonicalID,
			Vendor:          "anthropic",
			RawModel:        resolved.RawModel,
			BackendProvider: resolved.BackendProvider,
		},
		AppliedModel: agentapi.RuntimeModelRef{
			CanonicalID:     resolved.CanonicalID,
			Vendor:          "anthropic",
			RawModel:        resolved.RawModel,
			BackendProvider: resolved.BackendProvider,
		},
		BackendProvider: resolved.BackendProvider,
		ReloadCount:     1,
	})

	provider := &stubSelfKnowledgeProvider{
		agentName:     "IntegrationCat",
		modelName:     "gpt-4.1",
		providerName:  "openai",
		channelType:   "telegram",
		daemonStarted: time.Now().Add(-5 * time.Minute),
	}

	selfStatus := NewAgentSelfStatusTool(provider, holder)
	statusOutput, err := selfStatus.Execute(context.Background(), mustJSON(map[string]bool{"full": false}))
	if err != nil {
		t.Fatalf("agent_self_status returned error: %v", err)
	}
	if !strings.Contains(statusOutput, "--- Model State ---") {
		t.Fatalf("expected model state section, got:\n%s", statusOutput)
	}
	if !strings.Contains(statusOutput, "Configured: anthropic/claude-opus-4-6") {
		t.Fatalf("expected configured model to reflect switch, got:\n%s", statusOutput)
	}
	if !strings.Contains(statusOutput, "Applied: anthropic/claude-opus-4-6") {
		t.Fatalf("expected applied model to reflect switch, got:\n%s", statusOutput)
	}
	if !strings.Contains(statusOutput, "Backend: zen") {
		t.Fatalf("expected backend to reflect switch, got:\n%s", statusOutput)
	}
}

func TestIntegrationSelfRestartRequiresConfirm(t *testing.T) {
	tool := NewSelfRestartTool()

	cancelResult, err := tool.Execute(context.Background(), mustJSON(map[string]bool{"confirm": false}))
	if err != nil {
		t.Fatalf("confirm=false should not error, got: %v", err)
	}
	if !strings.Contains(cancelResult, "Restart cancelled: confirm parameter must be true") {
		t.Fatalf("expected cancellation message, got: %q", cancelResult)
	}

	if runtime.GOOS == "linux" {
		t.Skip("non-Linux unsupported-platform assertion is not applicable on Linux")
	}

	mock := &mockServiceManager{installed: true}
	nonLinuxTool := NewSelfRestartToolWithFactory(func() service.Manager { return mock })
	unsupportedResult, err := nonLinuxTool.Execute(context.Background(), mustJSON(map[string]bool{"confirm": true}))
	if err != nil {
		t.Fatalf("confirm=true on non-Linux should not error, got: %v", err)
	}
	if !strings.Contains(unsupportedResult, "self_restart is only supported on Linux with systemd") {
		t.Fatalf("expected unsupported platform message, got: %q", unsupportedResult)
	}
	if mock.restartCalls > 0 {
		t.Fatal("restart should not be attempted on non-Linux")
	}
}

func TestIntegrationModelSwitchUnknownModel(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "blackcat.yaml")

	input := `providers:
  openai:
    enabled: true
    model: gpt-4.1
  copilot:
    enabled: true
    model: gpt-5.3-codex
  gemini:
    enabled: true
    model: gemini-2.5-pro
  zen:
    enabled: true
    model: claude-3-5-sonnet
`
	if err := os.WriteFile(configPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input config: %v", err)
	}

	_ = NewConfigUpdateTool(configPath)

	cfg := &config.Config{}
	cfg.Providers.OpenAI.Enabled = true
	cfg.Providers.Copilot.Enabled = true
	cfg.Providers.Gemini.Enabled = true
	cfg.Providers.Zen.Enabled = true

	tool := NewModelSwitchTool(cfg)
	result, err := tool.Execute(context.Background(), mustJSON(map[string]string{"model": "totally-unknown-model-xyz"}))
	if err != nil {
		t.Fatalf("unknown model should degrade to fallback without error, got: %v", err)
	}
	if !strings.Contains(result, "providers.") {
		t.Fatalf("expected response to include config field path, got: %q", result)
	}
}
