package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Load reads configuration from the provided YAML file path,
// binds environment variables with BLACKCAT_ prefix,
// and applies defaults.
func Load(path string) (*Config, error) {
	// Get defaults first
	cfg := Defaults()

	// If path is provided, read the config file
	if path != "" {
		// Expand ~ to home directory
		if len(path) > 0 && path[0] == '~' {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			path = filepath.Join(home, path[1:])
		}

		// Read YAML file directly first
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		if err == nil && len(data) > 0 {
			// Unmarshal YAML directly into config
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
			log.Printf("Loaded config from: %s", path)
		}
	} else {
		// Auto-discovery when path is empty
		discoveredPath, found := discoverConfigPath()
		if found {
			data, err := os.ReadFile(discoveredPath)
			if err == nil && len(data) > 0 {
				if err := yaml.Unmarshal(data, cfg); err != nil {
					return nil, err
				}
				log.Printf("Loaded config from: %s (auto-discovered)", discoveredPath)
			}
		} else {
			log.Printf("No config file found; using defaults and environment variables")
		}
	}

	// Now apply environment variable overrides using viper
	v := viper.New()
	v.SetEnvPrefix("BLACKCAT")
	v.AutomaticEnv()
	v.SetConfigType("yaml")

	// Bind all environment variables
	bindEnvVars(v)

	// Create a temporary viper config with env vars only
	// to get any env var overrides
	envCfg := &Config{}
	if err := v.Unmarshal(envCfg); err == nil {
		// Override with non-empty env values
		mergeEnvOverrides(cfg, envCfg)
	}

	return cfg, nil
}

// mergeEnvOverrides applies non-empty/non-zero environment variable values to config.
func mergeEnvOverrides(cfg, envCfg *Config) {
	// Server
	if envCfg.Server.Addr != "" {
		cfg.Server.Addr = envCfg.Server.Addr
	}
	if envCfg.Server.Port != 0 {
		cfg.Server.Port = envCfg.Server.Port
	}

	// OpenCode
	if envCfg.OpenCode.Addr != "" {
		cfg.OpenCode.Addr = envCfg.OpenCode.Addr
	}
	if envCfg.OpenCode.Password != "" {
		cfg.OpenCode.Password = envCfg.OpenCode.Password
	}
	if !envCfg.OpenCode.Timeout.IsZero() {
		cfg.OpenCode.Timeout = envCfg.OpenCode.Timeout
	}

	// LLM
	if envCfg.LLM.Provider != "" {
		cfg.LLM.Provider = envCfg.LLM.Provider
	}
	if envCfg.LLM.Model != "" {
		cfg.LLM.Model = envCfg.LLM.Model
	}
	if envCfg.LLM.APIKey != "" {
		cfg.LLM.APIKey = envCfg.LLM.APIKey
	}
	if envCfg.LLM.BaseURL != "" {
		cfg.LLM.BaseURL = envCfg.LLM.BaseURL
	}
	if envCfg.LLM.Temperature != 0 {
		cfg.LLM.Temperature = envCfg.LLM.Temperature
	}
	if envCfg.LLM.MaxTokens != 0 {
		cfg.LLM.MaxTokens = envCfg.LLM.MaxTokens
	}

	// Channels
	if envCfg.Channels.Telegram.Enabled {
		cfg.Channels.Telegram.Enabled = true
	}
	if envCfg.Channels.Telegram.Token != "" {
		cfg.Channels.Telegram.Token = envCfg.Channels.Telegram.Token
	}
	if envCfg.Channels.Discord.Enabled {
		cfg.Channels.Discord.Enabled = true
	}
	if envCfg.Channels.Discord.Token != "" {
		cfg.Channels.Discord.Token = envCfg.Channels.Discord.Token
	}
	if envCfg.Channels.WhatsApp.Enabled {
		cfg.Channels.WhatsApp.Enabled = true
	}
	if envCfg.Channels.WhatsApp.Token != "" {
		cfg.Channels.WhatsApp.Token = envCfg.Channels.WhatsApp.Token
	}

	// Security
	if envCfg.Security.VaultPath != "" {
		cfg.Security.VaultPath = envCfg.Security.VaultPath
	}
	if len(envCfg.Security.DenyPatterns) > 0 {
		cfg.Security.DenyPatterns = envCfg.Security.DenyPatterns
	}
	if envCfg.Security.AutoPermit {
		cfg.Security.AutoPermit = true
	}

	// Memory
	if envCfg.Memory.FilePath != "" {
		cfg.Memory.FilePath = envCfg.Memory.FilePath
	}
	if envCfg.Memory.ConsolidationThreshold != 0 {
		cfg.Memory.ConsolidationThreshold = envCfg.Memory.ConsolidationThreshold
	}

	// Skills
	if envCfg.Skills.Dir != "" {
		cfg.Skills.Dir = envCfg.Skills.Dir
	}

	// Logging
	if envCfg.Logging.Level != "" {
		cfg.Logging.Level = envCfg.Logging.Level
	}
	if envCfg.Logging.Format != "" {
		cfg.Logging.Format = envCfg.Logging.Format
	}

	// OAuth
	if envCfg.OAuth.Copilot.Enabled {
		cfg.OAuth.Copilot.Enabled = true
	}
	if envCfg.OAuth.Copilot.ClientID != "" {
		cfg.OAuth.Copilot.ClientID = envCfg.OAuth.Copilot.ClientID
	}
	if envCfg.OAuth.Antigravity.Enabled {
		cfg.OAuth.Antigravity.Enabled = true
	}
	if envCfg.OAuth.Antigravity.AcceptedToS {
		cfg.OAuth.Antigravity.AcceptedToS = true
	}
	if envCfg.OAuth.Antigravity.ClientID != "" {
		cfg.OAuth.Antigravity.ClientID = envCfg.OAuth.Antigravity.ClientID
	}
	if envCfg.OAuth.Antigravity.ClientSecret != "" {
		cfg.OAuth.Antigravity.ClientSecret = envCfg.OAuth.Antigravity.ClientSecret
	}

	// Zen
	if envCfg.Zen.Enabled {
		cfg.Zen.Enabled = true
	}
	if envCfg.Zen.APIKey != "" {
		cfg.Zen.APIKey = envCfg.Zen.APIKey
	}
	if envCfg.Zen.BaseURL != "" {
		cfg.Zen.BaseURL = envCfg.Zen.BaseURL
	}
	if len(envCfg.Zen.Models) > 0 {
		cfg.Zen.Models = envCfg.Zen.Models
	}

	// Providers
	if envCfg.Providers.OpenAI.Enabled {
		cfg.Providers.OpenAI.Enabled = true
	}
	if envCfg.Providers.OpenAI.Model != "" {
		cfg.Providers.OpenAI.Model = envCfg.Providers.OpenAI.Model
	}
	if envCfg.Providers.OpenAI.APIKey != "" {
		cfg.Providers.OpenAI.APIKey = envCfg.Providers.OpenAI.APIKey
	}
	if envCfg.Providers.OpenAI.BaseURL != "" {
		cfg.Providers.OpenAI.BaseURL = envCfg.Providers.OpenAI.BaseURL
	}
	if envCfg.Providers.Copilot.Enabled {
		cfg.Providers.Copilot.Enabled = true
	}
	if envCfg.Providers.Copilot.Model != "" {
		cfg.Providers.Copilot.Model = envCfg.Providers.Copilot.Model
	}
	if envCfg.Providers.Antigravity.Enabled {
		cfg.Providers.Antigravity.Enabled = true
	}
	if envCfg.Providers.Antigravity.Model != "" {
		cfg.Providers.Antigravity.Model = envCfg.Providers.Antigravity.Model
	}
	if envCfg.Providers.Gemini.Enabled {
		cfg.Providers.Gemini.Enabled = true
	}
	if envCfg.Providers.Gemini.Model != "" {
		cfg.Providers.Gemini.Model = envCfg.Providers.Gemini.Model
	}
	if envCfg.Providers.Gemini.APIKey != "" {
		cfg.Providers.Gemini.APIKey = envCfg.Providers.Gemini.APIKey
	}
	if envCfg.Providers.Zen.Enabled {
		cfg.Providers.Zen.Enabled = true
	}
	if envCfg.Providers.Zen.Model != "" {
		cfg.Providers.Zen.Model = envCfg.Providers.Zen.Model
	}
}

// bindEnvVars binds individual config fields to environment variables.
func bindEnvVars(v *viper.Viper) {
	// Server
	v.BindEnv("server.addr", "BLACKCAT_SERVER_ADDR")
	v.BindEnv("server.port", "BLACKCAT_SERVER_PORT")

	// OpenCode
	v.BindEnv("opencode.addr", "BLACKCAT_OPENCODE_ADDR")
	v.BindEnv("opencode.password", "BLACKCAT_OPENCODE_PASSWORD")
	v.BindEnv("opencode.timeout", "BLACKCAT_OPENCODE_TIMEOUT")

	// LLM
	v.BindEnv("llm.provider", "BLACKCAT_LLM_PROVIDER")
	v.BindEnv("llm.model", "BLACKCAT_LLM_MODEL")
	v.BindEnv("llm.apiKey", "BLACKCAT_LLM_APIKEY")
	v.BindEnv("llm.baseURL", "BLACKCAT_LLM_BASEURL")
	v.BindEnv("llm.temperature", "BLACKCAT_LLM_TEMPERATURE")
	v.BindEnv("llm.maxTokens", "BLACKCAT_LLM_MAXTOKENS")

	// Channels
	v.BindEnv("channels.telegram.enabled", "BLACKCAT_CHANNELS_TELEGRAM_ENABLED")
	v.BindEnv("channels.telegram.token", "BLACKCAT_CHANNELS_TELEGRAM_TOKEN")
	v.BindEnv("channels.discord.enabled", "BLACKCAT_CHANNELS_DISCORD_ENABLED")
	v.BindEnv("channels.discord.token", "BLACKCAT_CHANNELS_DISCORD_TOKEN")
	v.BindEnv("channels.whatsapp.enabled", "BLACKCAT_CHANNELS_WHATSAPP_ENABLED")
	v.BindEnv("channels.whatsapp.token", "BLACKCAT_CHANNELS_WHATSAPP_TOKEN")

	// Security
	v.BindEnv("security.vaultPath", "BLACKCAT_SECURITY_VAULTPATH")
	v.BindEnv("security.autoPermit", "BLACKCAT_SECURITY_AUTOPERMIT")

	// Memory
	v.BindEnv("memory.filePath", "BLACKCAT_MEMORY_FILEPATH")
	v.BindEnv("memory.consolidationThreshold", "BLACKCAT_MEMORY_CONSOLIDATIONTHRESHOLD")

	// Skills
	v.BindEnv("skills.dir", "BLACKCAT_SKILLS_DIR")

	// Logging
	v.BindEnv("logging.level", "BLACKCAT_LOGGING_LEVEL")
	v.BindEnv("logging.format", "BLACKCAT_LOGGING_FORMAT")

	// OAuth
	v.BindEnv("oauth.copilot.enabled", "BLACKCAT_OAUTH_COPILOT_ENABLED")
	v.BindEnv("oauth.copilot.clientID", "BLACKCAT_OAUTH_COPILOT_CLIENTID")
	v.BindEnv("oauth.antigravity.enabled", "BLACKCAT_OAUTH_ANTIGRAVITY_ENABLED")
	v.BindEnv("oauth.antigravity.acceptedToS", "BLACKCAT_OAUTH_ANTIGRAVITY_ACCEPTEDTOS")
	v.BindEnv("oauth.antigravity.clientID", "BLACKCAT_OAUTH_ANTIGRAVITY_CLIENTID")
	v.BindEnv("oauth.antigravity.clientSecret", "BLACKCAT_OAUTH_ANTIGRAVITY_CLIENTSECRET")

	// Zen
	v.BindEnv("zen.enabled", "BLACKCAT_ZEN_ENABLED")
	v.BindEnv("zen.apiKey", "BLACKCAT_ZEN_APIKEY")
	v.BindEnv("zen.baseURL", "BLACKCAT_ZEN_BASEURL")

	// Providers
	v.BindEnv("providers.openai.enabled", "BLACKCAT_PROVIDERS_OPENAI_ENABLED")
	v.BindEnv("providers.openai.model", "BLACKCAT_PROVIDERS_OPENAI_MODEL")
	v.BindEnv("providers.openai.apiKey", "BLACKCAT_PROVIDERS_OPENAI_API_KEY")
	v.BindEnv("providers.openai.baseURL", "BLACKCAT_PROVIDERS_OPENAI_BASE_URL")
	v.BindEnv("providers.copilot.enabled", "BLACKCAT_PROVIDERS_COPILOT_ENABLED")
	v.BindEnv("providers.copilot.model", "BLACKCAT_PROVIDERS_COPILOT_MODEL")
	v.BindEnv("providers.antigravity.enabled", "BLACKCAT_PROVIDERS_ANTIGRAVITY_ENABLED")
	v.BindEnv("providers.antigravity.model", "BLACKCAT_PROVIDERS_ANTIGRAVITY_MODEL")
	v.BindEnv("providers.gemini.enabled", "BLACKCAT_PROVIDERS_GEMINI_ENABLED")
	v.BindEnv("providers.gemini.model", "BLACKCAT_PROVIDERS_GEMINI_MODEL")
	v.BindEnv("providers.gemini.apiKey", "BLACKCAT_PROVIDERS_GEMINI_APIKEY")
	v.BindEnv("providers.zen.enabled", "BLACKCAT_PROVIDERS_ZEN_ENABLED")
	v.BindEnv("providers.zen.model", "BLACKCAT_PROVIDERS_ZEN_MODEL")
}

// Defaults returns a Config with all default values filled in.
func Defaults() *Config {
	home, _ := os.UserHomeDir()
	vaultPath := filepath.Join(home, ".blackcat", "vault.json")

	return &Config{
		Server: ServerConfig{
			Addr: ":8080",
			Port: 8080,
		},
		OpenCode: OpenCodeConfig{
			Addr:    "http://127.0.0.1:4096",
			Timeout: Duration{30 * time.Minute},
		},
		LLM: LLMConfig{
			Temperature: 0.7,
			MaxTokens:   4096,
		},
		Channels: ChannelsConfig{
			Telegram: TelegramConfig{},
			Discord:  DiscordConfig{},
			WhatsApp: WhatsAppConfig{},
		},
		Security: SecurityConfig{
			VaultPath:  vaultPath,
			AutoPermit: false,
		},
		Memory: MemoryConfig{
			FilePath:               "MEMORY.md",
			ConsolidationThreshold: 50,
		},
		MCP: MCPConfig{
			Servers: []MCPServerConfig{},
		},
		Skills: SkillsConfig{
			Dir: "skills/",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		OAuth: OAuthConfig{
			Copilot: CopilotOAuthConfig{
				ClientID: "01ab8ac9400c4e429b23", // VS Code client ID
			},
			Antigravity: AntigravityOAuthConfig{
				ClientID:     "", // Set via BLACKCAT_OAUTH_ANTIGRAVITY_CLIENTID
				ClientSecret: "", // Set via BLACKCAT_OAUTH_ANTIGRAVITY_CLIENTSECRET
			},
		},
		Zen: ZenConfig{
			BaseURL: "https://api.opencode.ai/v1",
		},
		Providers: ProvidersConfig{},
	}
}

// discoverConfigPath searches standard locations for a config file.
// Returns the path and whether a file was found.
// Search order:
//  1. ./blackcat.yaml (current working directory)
//  2. ~/.config/blackcat/config.yaml
//  3. ~/.blackcat/config.yaml
//  4. /etc/blackcat/config.yaml
func discoverConfigPath() (string, bool) {
	paths := []string{
		"blackcat.yaml",
	}

	// Add paths with home directory expansion
	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ".config", "blackcat", "config.yaml"))
		paths = append(paths, filepath.Join(home, ".blackcat", "config.yaml"))
	}

	// Add system-wide path
	paths = append(paths, filepath.Join("/etc", "blackcat", "config.yaml"))

	// Check each path in order
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}

	return "", false
}

// Save writes cfg to the provided YAML file path.
// It creates the parent directory (mode 0700) if it does not exist,
// then writes the marshalled YAML with mode 0600.
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("config: create directory: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}
