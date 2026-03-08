package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	// Check server defaults
	if cfg.Server.Addr != ":8080" {
		t.Errorf("Server.Addr = %q, want %q", cfg.Server.Addr, ":8080")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 8080)
	}

	// Check OpenCode defaults
	if cfg.OpenCode.Addr != "http://127.0.0.1:4096" {
		t.Errorf("OpenCode.Addr = %q, want %q", cfg.OpenCode.Addr, "http://127.0.0.1:4096")
	}
	if cfg.OpenCode.Timeout.Duration != 30*time.Minute {
		t.Errorf("OpenCode.Timeout = %v, want %v", cfg.OpenCode.Timeout, Duration{30 * time.Minute})
	}

	// Check LLM defaults
	if cfg.LLM.Temperature != 0.7 {
		t.Errorf("LLM.Temperature = %f, want %f", cfg.LLM.Temperature, 0.7)
	}
	if cfg.LLM.MaxTokens != 4096 {
		t.Errorf("LLM.MaxTokens = %d, want %d", cfg.LLM.MaxTokens, 4096)
	}

	// Check Security defaults
	if cfg.Security.AutoPermit != false {
		t.Errorf("Security.AutoPermit = %v, want false", cfg.Security.AutoPermit)
	}

	// Check Memory defaults
	if cfg.Memory.FilePath != "MEMORY.md" {
		t.Errorf("Memory.FilePath = %q, want %q", cfg.Memory.FilePath, "MEMORY.md")
	}
	if cfg.Memory.ConsolidationThreshold != 50 {
		t.Errorf("Memory.ConsolidationThreshold = %d, want %d", cfg.Memory.ConsolidationThreshold, 50)
	}

	// Check Skills defaults
	if cfg.Skills.Dir != "skills/" {
		t.Errorf("Skills.Dir = %q, want %q", cfg.Skills.Dir, "skills/")
	}

	// Check Logging defaults
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "info")
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("Logging.Format = %q, want %q", cfg.Logging.Format, "text")
	}
}

func TestLoadFromYAML(t *testing.T) {
	// Create a temporary YAML file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	yaml := `
server:
  addr: ":9090"
  port: 9090
opencode:
  addr: "http://custom:5000"
llm:
  provider: "openai"
  model: "gpt-4"
  temperature: 0.5
  maxTokens: 2048
memory:
  filePath: "custom_memory.md"
  consolidationThreshold: 100
logging:
  level: "debug"
  format: "json"
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check loaded values
	if cfg.Server.Addr != ":9090" {
		t.Errorf("Server.Addr = %q, want %q", cfg.Server.Addr, ":9090")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 9090)
	}
	if cfg.OpenCode.Addr != "http://custom:5000" {
		t.Errorf("OpenCode.Addr = %q, want %q", cfg.OpenCode.Addr, "http://custom:5000")
	}
	if cfg.LLM.Provider != "openai" {
		t.Errorf("LLM.Provider = %q, want %q", cfg.LLM.Provider, "openai")
	}
	if cfg.LLM.Model != "gpt-4" {
		t.Errorf("LLM.Model = %q, want %q", cfg.LLM.Model, "gpt-4")
	}
	if cfg.LLM.Temperature != 0.5 {
		t.Errorf("LLM.Temperature = %f, want %f", cfg.LLM.Temperature, 0.5)
	}
	if cfg.LLM.MaxTokens != 2048 {
		t.Errorf("LLM.MaxTokens = %d, want %d", cfg.LLM.MaxTokens, 2048)
	}
	if cfg.Memory.FilePath != "custom_memory.md" {
		t.Errorf("Memory.FilePath = %q, want %q", cfg.Memory.FilePath, "custom_memory.md")
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "debug")
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Logging.Format = %q, want %q", cfg.Logging.Format, "json")
	}
}

func TestLoadWithEnvironmentOverride(t *testing.T) {
	// Create a temporary YAML file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	yaml := `
opencode:
  addr: "http://yaml:4096"
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Set environment variable
	os.Setenv("BLACKCAT_OPENCODE_ADDR", "http://env:5000")
	defer os.Unsetenv("BLACKCAT_OPENCODE_ADDR")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Environment variable should override YAML
	if cfg.OpenCode.Addr != "http://env:5000" {
		t.Errorf("OpenCode.Addr = %q, want %q (env override)", cfg.OpenCode.Addr, "http://env:5000")
	}
}

func TestLoadNonexistentFile(t *testing.T) {
	// Load from non-existent explicit path should return an error
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent explicit config path, got nil")
	}
}

func TestWatch(t *testing.T) {
	// Create a temporary YAML file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	yaml := `
server:
  addr: ":8080"
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Set up callback
	var mu sync.Mutex
	var reloadCount int
	var lastConfig *Config

	callback := func(cfg *Config) {
		mu.Lock()
		reloadCount++
		lastConfig = cfg
		mu.Unlock()
	}

	// Start watching
	watcher, err := Watch(configPath, callback)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}
	defer watcher.Stop()

	// Wait a bit for watcher to start
	time.Sleep(100 * time.Millisecond)

	// Modify the config file
	newYaml := `
server:
  addr: ":9090"
logging:
  level: "debug"
`
	if err := os.WriteFile(configPath, []byte(newYaml), 0o644); err != nil {
		t.Fatalf("Failed to write updated config file: %v", err)
	}

	// Wait for debounce and callback
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	if reloadCount == 0 {
		t.Errorf("callback not called, reloadCount = %d, want > 0", reloadCount)
	}
	if lastConfig != nil && lastConfig.Server.Addr != ":9090" {
		t.Errorf("Server.Addr = %q, want %q", lastConfig.Server.Addr, ":9090")
	}
	mu.Unlock()
}

func TestWatchStop(t *testing.T) {
	// Create a temporary YAML file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	yaml := `
server:
  addr: ":8080"
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	watcher, err := Watch(configPath, func(cfg *Config) {})
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	// Should be able to stop without error
	if err := watcher.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	// Second stop should also work
	if err := watcher.Stop(); err != nil {
		t.Errorf("Stop again failed: %v", err)
	}
}

func TestChannelsConfig(t *testing.T) {
	// Create a temporary YAML file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	yaml := `
channels:
  telegram:
    enabled: true
    token: "telegram-secret"
  discord:
    enabled: false
  whatsapp:
    enabled: true
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !cfg.Channels.Telegram.Enabled {
		t.Errorf("Channels.Telegram.Enabled = %v, want true", cfg.Channels.Telegram.Enabled)
	}
	if cfg.Channels.Telegram.Token != "telegram-secret" {
		t.Errorf("Channels.Telegram.Token = %q, want %q", cfg.Channels.Telegram.Token, "telegram-secret")
	}
	if cfg.Channels.Discord.Enabled {
		t.Errorf("Channels.Discord.Enabled = %v, want false", cfg.Channels.Discord.Enabled)
	}
	if !cfg.Channels.WhatsApp.Enabled {
		t.Errorf("Channels.WhatsApp.Enabled = %v, want true", cfg.Channels.WhatsApp.Enabled)
	}
}

func TestMCPConfig(t *testing.T) {
	// Create a temporary YAML file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	yaml := `
mcp:
  servers:
    - name: "my-server"
      command: "/usr/local/bin/server"
      args: ["--port", "9000"]
      env:
        VAR1: "value1"
        VAR2: "value2"
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.MCP.Servers) != 1 {
		t.Errorf("MCP.Servers length = %d, want 1", len(cfg.MCP.Servers))
	}

	server := cfg.MCP.Servers[0]
	if server.Name != "my-server" {
		t.Errorf("MCP.Servers[0].Name = %q, want %q", server.Name, "my-server")
	}
	if server.Command != "/usr/local/bin/server" {
		t.Errorf("MCP.Servers[0].Command = %q, want %q", server.Command, "/usr/local/bin/server")
	}
	if len(server.Args) != 2 || server.Args[0] != "--port" || server.Args[1] != "9000" {
		t.Errorf("MCP.Servers[0].Args = %v, want [--port 9000]", server.Args)
	}
	if server.Env["VAR1"] != "value1" {
		t.Errorf("MCP.Servers[0].Env[VAR1] = %q, want %q", server.Env["VAR1"], "value1")
	}
}

func TestWhatsAppAllowFrom(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	yaml := `
channels:
  whatsapp:
    enabled: true
    token: "file:test.db"
    allowFrom:
      - "+628123456789"
      - "+6281234567890"
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !cfg.Channels.WhatsApp.Enabled {
		t.Error("WhatsApp.Enabled = false, want true")
	}
	if len(cfg.Channels.WhatsApp.AllowFrom) != 2 {
		t.Fatalf("WhatsApp.AllowFrom length = %d, want 2", len(cfg.Channels.WhatsApp.AllowFrom))
	}
	if cfg.Channels.WhatsApp.AllowFrom[0] != "+628123456789" {
		t.Errorf("WhatsApp.AllowFrom[0] = %q, want %q", cfg.Channels.WhatsApp.AllowFrom[0], "+628123456789")
	}
	if cfg.Channels.WhatsApp.AllowFrom[1] != "+6281234567890" {
		t.Errorf("WhatsApp.AllowFrom[1] = %q, want %q", cfg.Channels.WhatsApp.AllowFrom[1], "+6281234567890")
	}
}

func TestWhatsAppAllowFromEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	yaml := `
channels:
  whatsapp:
    enabled: true
    token: "file:test.db"
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Channels.WhatsApp.AllowFrom) != 0 {
		t.Errorf("WhatsApp.AllowFrom length = %d, want 0", len(cfg.Channels.WhatsApp.AllowFrom))
	}
}

// --- Phase 4 Wave 1: Roles & RTK Tests ---

func TestValidatePopulatesDefaultRoles(t *testing.T) {
	cfg := &Config{} // empty roles
	cfg.Validate()

	if len(cfg.Roles) != 7 {
		t.Fatalf("Roles length = %d, want 7", len(cfg.Roles))
	}

	expectedNames := []string{"phantom", "astrology", "wizard", "artist", "scribe", "explorer", "oracle"}
	for i, name := range expectedNames {
		if cfg.Roles[i].Name != name {
			t.Errorf("Roles[%d].Name = %q, want %q", i, cfg.Roles[i].Name, name)
		}
	}
}

func TestValidatePreservesCustomRoles(t *testing.T) {
	custom := []RoleConfig{
		{Name: "mybot", Model: "gpt-5", Temperature: 0.5, Keywords: []string{"hello"}, Priority: 1},
	}
	cfg := &Config{Roles: custom}
	cfg.Validate()

	if len(cfg.Roles) != 1 {
		t.Fatalf("Roles length = %d, want 1 (custom not replaced)", len(cfg.Roles))
	}
	if cfg.Roles[0].Name != "mybot" {
		t.Errorf("Roles[0].Name = %q, want %q", cfg.Roles[0].Name, "mybot")
	}
}

func TestValidateDeepDuplicateRoleName(t *testing.T) {
	cfg := &Config{
		Roles: []RoleConfig{
			{Name: "wizard", Priority: 10},
			{Name: "wizard", Priority: 20},
		},
	}

	err := ValidateDeep(cfg)
	if err == nil {
		t.Fatal("expected error for duplicate role name, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error = %q, want it to contain 'duplicate'", err.Error())
	}
}

func TestValidateDeepRoleTemperatureOutOfRange(t *testing.T) {
	cfg := &Config{
		Roles: []RoleConfig{
			{Name: "hot", Temperature: 3.0, Priority: 10},
		},
	}

	err := ValidateDeep(cfg)
	if err == nil {
		t.Fatal("expected error for temperature 3.0, got nil")
	}
	if !strings.Contains(err.Error(), "temperature") {
		t.Errorf("error = %q, want it to contain 'temperature'", err.Error())
	}
}

func TestValidateRTKDefaultCommands(t *testing.T) {
	cfg := &Config{
		RTK: RTKConfig{Enabled: true}, // Commands empty
	}
	cfg.Validate()

	if len(cfg.RTK.Commands) == 0 {
		t.Fatal("RTK.Commands should be populated when Enabled=true and Commands empty")
	}

	// Verify a few expected defaults are present
	expected := map[string]bool{"cargo": false, "git": false, "docker": false, "tsc": false}
	for _, cmd := range cfg.RTK.Commands {
		if _, ok := expected[cmd]; ok {
			expected[cmd] = true
		}
	}
	for cmd, found := range expected {
		if !found {
			t.Errorf("RTK.Commands missing expected default %q", cmd)
		}
	}
}

// --- Phase 5: Provider symmetry and override diagnostics ---

func TestLoadWithOpenAIProviderEnvOverride(t *testing.T) {
	// Create a YAML with providers.openai.model set
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	yamlContent := `
providers:
  openai:
    enabled: true
    model: "gpt-4o"
    apiKey: "sk-yaml-key"
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Set environment overrides
	t.Setenv("BLACKCAT_PROVIDERS_OPENAI_MODEL", "gpt-5.2")
	t.Setenv("BLACKCAT_PROVIDERS_OPENAI_ENABLED", "true")
	t.Setenv("BLACKCAT_PROVIDERS_OPENAI_API_KEY", "sk-env-key")
	t.Setenv("BLACKCAT_PROVIDERS_OPENAI_BASE_URL", "https://custom.openai.com/v1")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Environment variables should override YAML values
	if cfg.Providers.OpenAI.Model != "gpt-5.2" {
		t.Errorf("Providers.OpenAI.Model = %q, want %q (env override)", cfg.Providers.OpenAI.Model, "gpt-5.2")
	}
	if !cfg.Providers.OpenAI.Enabled {
		t.Errorf("Providers.OpenAI.Enabled = %v, want true (env override)", cfg.Providers.OpenAI.Enabled)
	}
	if cfg.Providers.OpenAI.APIKey != "sk-env-key" {
		t.Errorf("Providers.OpenAI.APIKey = %q, want %q (env override)", cfg.Providers.OpenAI.APIKey, "sk-env-key")
	}
	if cfg.Providers.OpenAI.BaseURL != "https://custom.openai.com/v1" {
		t.Errorf("Providers.OpenAI.BaseURL = %q, want %q (env override)", cfg.Providers.OpenAI.BaseURL, "https://custom.openai.com/v1")
	}
}

func TestAuthoritativeProviderFieldBeatsLegacyLLMModel(t *testing.T) {
	// Create a YAML with both legacy llm.model and providers.openai.model
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	yamlContent := `
llm:
  provider: "openai"
  model: "gpt-4"
providers:
  openai:
    enabled: true
    model: "gpt-5.2"
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// The authoritative provider field should have its own value
	if cfg.Providers.OpenAI.Model != "gpt-5.2" {
		t.Errorf("Providers.OpenAI.Model = %q, want %q (authoritative)", cfg.Providers.OpenAI.Model, "gpt-5.2")
	}
	if !cfg.Providers.OpenAI.Enabled {
		t.Errorf("Providers.OpenAI.Enabled = %v, want true", cfg.Providers.OpenAI.Enabled)
	}

	// Legacy fields should still be populated for backward compat
	if cfg.LLM.Provider != "openai" {
		t.Errorf("LLM.Provider = %q, want %q (legacy compat)", cfg.LLM.Provider, "openai")
	}
	if cfg.LLM.Model != "gpt-4" {
		t.Errorf("LLM.Model = %q, want %q (legacy compat)", cfg.LLM.Model, "gpt-4")
	}

	// Env override on provider field should NOT affect legacy field
	t.Setenv("BLACKCAT_PROVIDERS_OPENAI_MODEL", "gpt-5.1-codex")

	cfg2, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load (with env) failed: %v", err)
	}

	if cfg2.Providers.OpenAI.Model != "gpt-5.1-codex" {
		t.Errorf("Providers.OpenAI.Model = %q, want %q (env override)", cfg2.Providers.OpenAI.Model, "gpt-5.1-codex")
	}
	// Legacy llm.model should still reflect the YAML value, not the env override
	if cfg2.LLM.Model != "gpt-4" {
		t.Errorf("LLM.Model = %q, want %q (legacy should not be affected by provider env)", cfg2.LLM.Model, "gpt-4")
	}
}

func TestOverrideDiagnosticsDefault(t *testing.T) {
	cfg := Defaults()

	diag := GetFieldSourceDiagnostics(cfg, "llm.provider")
	if diag.Source != "default" {
		t.Errorf("Source = %q, want %q", diag.Source, "default")
	}
	if diag.EnvVar != "BLACKCAT_LLM_PROVIDER" {
		t.Errorf("EnvVar = %q, want %q", diag.EnvVar, "BLACKCAT_LLM_PROVIDER")
	}
}

func TestOverrideDiagnosticsYAML(t *testing.T) {
	cfg := Defaults()
	cfg.Providers.OpenAI.Model = "gpt-5.2"

	diag := GetFieldSourceDiagnostics(cfg, "providers.openai.model")
	if diag.Source != "yaml" {
		t.Errorf("Source = %q, want %q", diag.Source, "yaml")
	}
	if diag.YAMLValue != "gpt-5.2" {
		t.Errorf("YAMLValue = %q, want %q", diag.YAMLValue, "gpt-5.2")
	}
}

func TestOverrideDiagnosticsEnv(t *testing.T) {
	cfg := Defaults()
	cfg.Providers.OpenAI.Model = "gpt-5.2"

	t.Setenv("BLACKCAT_PROVIDERS_OPENAI_MODEL", "gpt-5.1-codex")

	diag := GetFieldSourceDiagnostics(cfg, "providers.openai.model")
	if diag.EnvValue != "gpt-5.1-codex" {
		t.Errorf("EnvValue = %q, want %q", diag.EnvValue, "gpt-5.1-codex")
	}
	// Both YAML and env are set, and they differ, so env wins
	if diag.Source != "mixed-env-wins" {
		t.Errorf("Source = %q, want %q", diag.Source, "mixed-env-wins")
	}
}

func TestOverrideDiagnosticsUnknownField(t *testing.T) {
	cfg := Defaults()

	diag := GetFieldSourceDiagnostics(cfg, "providers.unknown.model")
	if diag.Field != "providers.unknown.model" {
		t.Fatalf("Field = %q; want %q", diag.Field, "providers.unknown.model")
	}
	if diag.EnvVar != "" {
		t.Fatalf("EnvVar = %q; want empty for unknown field", diag.EnvVar)
	}
	if diag.Source != "default" {
		t.Fatalf("Source = %q; want %q", diag.Source, "default")
	}
}

func TestOverrideDiagnosticsEnvSameAsYAML(t *testing.T) {
	cfg := Defaults()
	cfg.Providers.OpenAI.Model = "gpt-5.2"

	t.Setenv("BLACKCAT_PROVIDERS_OPENAI_MODEL", "gpt-5.2")

	diag := GetFieldSourceDiagnostics(cfg, "providers.openai.model")
	if diag.Source != "env" {
		t.Fatalf("Source = %q; want %q", diag.Source, "env")
	}
	if diag.EnvValue != "gpt-5.2" {
		t.Fatalf("EnvValue = %q; want %q", diag.EnvValue, "gpt-5.2")
	}
}
