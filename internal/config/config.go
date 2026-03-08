package config

import "time"

// Config is the root configuration struct for BlackCat.
// It covers all modules and can be loaded from YAML, environment variables, and defaults.
type Config struct {
	Server       ServerConfig       `yaml:"server"`       // HTTP listen addr, port
	OpenCode     OpenCodeConfig     `yaml:"opencode"`     // addr, password, timeout
	LLM          LLMConfig          `yaml:"llm"`          // provider, model, apiKey, baseURL, temperature
	Channels     ChannelsConfig     `yaml:"channels"`     // telegram, discord, whatsapp sub-configs
	Security     SecurityConfig     `yaml:"security"`     // vault path, deny patterns, auto-permit
	Memory       MemoryConfig       `yaml:"memory"`       // file path, consolidation threshold
	MCP          MCPConfig          `yaml:"mcp"`          // server/client configs
	Skills       SkillsConfig       `yaml:"skills"`       // skills directory path
	Logging      LoggingConfig      `yaml:"logging"`      // level, format (json/text)
	OAuth        OAuthConfig        `yaml:"oauth"`        // OAuth provider settings (Copilot, Antigravity)
	Zen          ZenConfig          `yaml:"zen"`          // Zen Coding Plan settings
	Providers    ProvidersConfig    `yaml:"providers"`    // Per-provider enable/model overrides
	Dashboard    DashboardConfig    `yaml:"dashboard"`    // Dashboard settings
	Scheduler    SchedulerConfig    `yaml:"scheduler"`    // Scheduled jobs configuration
	Orchestrator OrchestratorConfig `yaml:"orchestrator"` // Orchestrator settings
	Session      SessionConfig      `yaml:"session"`      // Session store configuration
	Rules        RulesConfig        `yaml:"rules"`        // Rules directory
	Profiles     ProfilesConfig     `yaml:"profiles"`     // Profiles directory
	Agent        AgentConfig        `yaml:"agent"`        // Agent persona and personalization settings
	RateLimit    RateLimitConfig    `yaml:"rateLimit"`    // Per-user rate limiting
	Whisper      WhisperConfig      `yaml:"whisper"`      // Groq Whisper voice-to-text configuration
	Budget       BudgetConfig       `yaml:"budget"`       // Per-user spend limits
	Roles        []RoleConfig       `yaml:"roles"`        // Agent role definitions
	RTK          RTKConfig          `yaml:"rtk"`          // RTK token optimization settings

}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Addr string `yaml:"addr"` // e.g., ":8080"
	Port int    `yaml:"port"` // e.g., 8080
}

// OpenCodeConfig holds OpenCode agent settings.
type OpenCodeConfig struct {
	Addr     string   `yaml:"addr"`     // e.g., "http://127.0.0.1:4096"
	Password string   `yaml:"password"` // Set via env or vault
	Timeout  Duration `yaml:"timeout"`  // e.g., "30m"
}

// LLMConfig holds LLM provider settings.
type LLMConfig struct {
	Provider         string   `yaml:"provider"`         // LEGACY: read-only compatibility, use providers.*.model for new code
	Model            string   `yaml:"model"`            // LEGACY: read-only compatibility, use providers.*.model for new code
	APIKey           string   `yaml:"apiKey"`           // Set via env or vault
	BaseURL          string   `yaml:"baseURL"`          // Optional: custom base URL
	Temperature      float64  `yaml:"temperature"`      // 0.0 to 2.0
	MaxTokens        int      `yaml:"maxTokens"`        // Max tokens per response
	MaxContextTokens int      `yaml:"maxContextTokens"` // Max context window tokens (0 = disabled, default 80000)
	Fallback         []string `yaml:"fallback"`         // Fallback model chain
}

// ChannelsConfig holds communication channel settings.
type ChannelsConfig struct {
	Telegram TelegramConfig `yaml:"telegram"`
	Discord  DiscordConfig  `yaml:"discord"`
	WhatsApp WhatsAppConfig `yaml:"whatsapp"`
}

// TelegramConfig holds Telegram-specific settings.
type TelegramConfig struct {
	Enabled bool   `yaml:"enabled"`
	Token   string `yaml:"token"` // Set via env or vault
}

// DiscordConfig holds Discord-specific settings.
type DiscordConfig struct {
	Enabled bool   `yaml:"enabled"`
	Token   string `yaml:"token"` // Set via env or vault
}

// WhatsAppConfig holds WhatsApp-specific settings.
type WhatsAppConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Token     string   `yaml:"token"`     // Set via env or vault
	AllowFrom []string `yaml:"allowFrom"` // Phone whitelist (E.164); empty = allow all
}

// SecurityConfig holds security-related settings.
type SecurityConfig struct {
	VaultPath    string           `yaml:"vaultPath"`    // e.g., "~/.blackcat/vault.json"
	DenyPatterns []string         `yaml:"denyPatterns"` // Patterns to deny
	AutoPermit   bool             `yaml:"autoPermit"`   // Auto-permit requests
	Guardrails   GuardrailsConfig `yaml:"guardrails"`
	HITL         HITLConfig       `yaml:"hitl"`
}

// MemoryConfig holds memory consolidation settings.
type MemoryConfig struct {
	FilePath               string           `yaml:"filePath"`               // e.g., "MEMORY.md"
	ConsolidationThreshold int              `yaml:"consolidationThreshold"` // Threshold for consolidation
	Store                  string           `yaml:"store"`                  // "sqlite" (default) or "file"
	SQLitePath             string           `yaml:"sqlitePath"`             // e.g., "~/.blackcat/memory.db"
	Embedding              EmbeddingConfig  `yaml:"embedding"`
	CoreMemory             CoreMemoryConfig `yaml:"coreMemory"`
	MaxArchival            int              `yaml:"maxArchival"` // 0 = unlimited
}

// EmbeddingConfig holds settings for the embedding provider.
type EmbeddingConfig struct {
	Provider string `yaml:"provider"` // e.g., "openai"
	Model    string `yaml:"model"`    // e.g., "text-embedding-3-small"
	APIKey   string `yaml:"apiKey"`   // Set via env or vault
	BaseURL  string `yaml:"baseURL"`  // Optional custom endpoint
}

// CoreMemoryConfig holds settings for the per-user core memory store.
type CoreMemoryConfig struct {
	MaxEntries  int `yaml:"maxEntries"`  // default 20
	MaxValueLen int `yaml:"maxValueLen"` // default 500
}

// InputGuardrailConfig holds settings for the input guardrail.
type InputGuardrailConfig struct {
	Enabled        bool     `yaml:"enabled"`
	CustomPatterns []string `yaml:"customPatterns"`
}

// ToolGuardrailConfig holds settings for the tool guardrail.
type ToolGuardrailConfig struct {
	Enabled                 bool     `yaml:"enabled"`
	RequireApprovalPatterns []string `yaml:"requireApproval"`
}

// OutputGuardrailConfig holds settings for the output guardrail.
type OutputGuardrailConfig struct {
	Enabled bool `yaml:"enabled"`
}

// GuardrailsConfig holds the full guardrails pipeline configuration.
type GuardrailsConfig struct {
	Input  InputGuardrailConfig  `yaml:"input"`
	Tool   ToolGuardrailConfig   `yaml:"tool"`
	Output OutputGuardrailConfig `yaml:"output"`
}

// HITLConfig holds human-in-the-loop approval settings.
type HITLConfig struct {
	Enabled        bool `yaml:"enabled"`
	TimeoutMinutes int  `yaml:"timeoutMinutes"`
}

// MCPConfig holds MCP server configurations.
type MCPConfig struct {
	Servers []MCPServerConfig `yaml:"servers"`
}

// MCPServerConfig holds a single MCP server configuration.
type MCPServerConfig struct {
	Name    string            `yaml:"name"`    // e.g., "my-mcp-server"
	Command string            `yaml:"command"` // e.g., "/usr/local/bin/my-server"
	Args    []string          `yaml:"args"`    // Command arguments
	Env     map[string]string `yaml:"env"`     // Environment variables
}

// SkillsConfig holds skills directory settings.
type SkillsConfig struct {
	Dir                  string `yaml:"dir"`
	MarketplaceDir       string `yaml:"marketplace_dir"`
	AllowExternalInstall bool   `yaml:"allow_external_install"`
	MaxSkillsInPrompt    int    `yaml:"max_skills_in_prompt"`
	MaxSkillFileBytes    int    `yaml:"max_skill_file_bytes"`
}

// WhisperConfig holds Groq Whisper voice-to-text configuration.
type WhisperConfig struct {
	Enabled       bool   `yaml:"enabled" mapstructure:"enabled"`
	GroqAPIKey    string `yaml:"groqApiKey" mapstructure:"groqApiKey"`
	Model         string `yaml:"model" mapstructure:"model"`
	MaxFileSizeMB int    `yaml:"maxFileSizeMb" mapstructure:"maxFileSizeMb"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string `yaml:"level"`  // e.g., "debug", "info", "warn", "error"
	Format string `yaml:"format"` // e.g., "json", "text"
}

// --- Phase 2: OAuth, Zen, and Provider Config ---

// OAuthConfig holds OAuth settings for providers that use OAuth authentication.
type OAuthConfig struct {
	Copilot     CopilotOAuthConfig     `yaml:"copilot"`
	Antigravity AntigravityOAuthConfig `yaml:"antigravity"`
}

// CopilotOAuthConfig holds GitHub Copilot device code flow settings.
type CopilotOAuthConfig struct {
	Enabled  bool   `yaml:"enabled"`
	ClientID string `yaml:"clientID"` // Defaults to VS Code client ID
}

// AntigravityOAuthConfig holds Google Antigravity PKCE flow settings.
type AntigravityOAuthConfig struct {
	Enabled      bool   `yaml:"enabled"`
	AcceptedToS  bool   `yaml:"acceptedToS"` // Must accept ToS risk to enable
	ClientID     string `yaml:"clientID"`
	ClientSecret string `yaml:"clientSecret"`
}

// ZenConfig holds Zen Coding Plan settings.
type ZenConfig struct {
	Enabled bool     `yaml:"enabled"`
	APIKey  string   `yaml:"apiKey"`  // Set via env or vault
	BaseURL string   `yaml:"baseURL"` // Zen API base URL
	Models  []string `yaml:"models"`  // Curated model list
}

// ProvidersConfig holds per-provider enable/model settings.
type ProvidersConfig struct {
	OpenAI      OpenAIProviderConfig      `yaml:"openai"`
	Copilot     CopilotProviderConfig     `yaml:"copilot"`
	Antigravity AntigravityProviderConfig `yaml:"antigravity"`
	Gemini      GeminiProviderConfig      `yaml:"gemini"`
	Zen         ZenProviderConfig         `yaml:"zen"`
}

// OpenAIProviderConfig holds OpenAI / Codex LLM provider settings.
type OpenAIProviderConfig struct {
	Enabled bool   `yaml:"enabled"`
	Model   string `yaml:"model"`   // e.g., "gpt-5.1-codex", "gpt-4o"
	APIKey  string `yaml:"apiKey"`  // OpenAI API key (or via vault/env)
	BaseURL string `yaml:"baseURL"` // Optional custom endpoint
}

// CopilotProviderConfig holds GitHub Copilot LLM provider settings.
type CopilotProviderConfig struct {
	Enabled bool   `yaml:"enabled"`
	Model   string `yaml:"model"` // e.g., "gpt-4o"
}

// AntigravityProviderConfig holds Google Antigravity LLM provider settings.
type AntigravityProviderConfig struct {
	Enabled bool   `yaml:"enabled"`
	Model   string `yaml:"model"` // e.g., "gemini-2.5-pro"
}

// GeminiProviderConfig holds Google Gemini official LLM provider settings.
type GeminiProviderConfig struct {
	Enabled bool   `yaml:"enabled"`
	Model   string `yaml:"model"`  // e.g., "gemini-1.5-pro"
	APIKey  string `yaml:"apiKey"` // Google API key
}

// ZenProviderConfig holds Zen Coding Plan LLM provider settings.
type ZenProviderConfig struct {
	Enabled bool   `yaml:"enabled"`
	Model   string `yaml:"model"` // From curated list
}

// --- Phase 3: Dashboard, Scheduler, Orchestrator, Session, Rules, Profiles ---

// DashboardConfig holds dashboard server settings.
type DashboardConfig struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`  // default: ":8081"
	Token   string `yaml:"token"` // auth token
}

// DeliverConfig specifies where a scheduled job sends its output.
type DeliverConfig struct {
	Channel   string `yaml:"channel"`   // "whatsapp", "telegram", "discord"
	ChannelID string `yaml:"channelId"` // chat/channel ID to send to
	Message   string `yaml:"message"`   // message text to deliver (if empty, delivers command stdout)
}

// ScheduledJob represents a single scheduled job configuration.
type ScheduledJob struct {
	Name     string         `yaml:"name"`
	Schedule string         `yaml:"schedule"` // cron expression
	Command  string         `yaml:"command"`
	Enabled  bool           `yaml:"enabled"`
	Deliver  *DeliverConfig `yaml:"deliver,omitempty"`
}

// SchedulerConfig holds scheduler configuration with a list of jobs.
type SchedulerConfig struct {
	Enabled bool           `yaml:"enabled"`
	Jobs    []ScheduledJob `yaml:"jobs"`
}

// OrchestratorConfig holds orchestrator settings for sub-agent coordination.
type OrchestratorConfig struct {
	MaxConcurrent   int      `yaml:"max_concurrent"`    // default: 5, hard cap: 10
	SubAgentTimeout Duration `yaml:"sub_agent_timeout"` // default: 5m
}

// SessionConfig holds session store configuration.
type SessionConfig struct {
	Enabled    bool   `yaml:"enabled"`
	StoreDir   string `yaml:"store_dir"`   // default: ~/.blackcat/sessions
	MaxHistory int    `yaml:"max_history"` // default: 50
}

// RulesConfig holds the directory for conditional rule .md files.
type RulesConfig struct {
	Dir string `yaml:"dir"`
}

// ProfilesConfig holds the directory for custom agent profile .md files.
type ProfilesConfig struct {
	Dir string `yaml:"dir"`
}

// AgentConfig holds agent persona and personalization settings.
type AgentConfig struct {
	Name       string `yaml:"name"`       // Agent display name, e.g. "Interstellar"
	Greeting   string `yaml:"greeting"`   // First-time greeting message
	Language   string `yaml:"language"`   // Response language, e.g. "id" (Indonesian) or "en"
	Tone       string `yaml:"tone"`       // Response tone: "friendly", "professional", "casual"
	AckMessage string `yaml:"ackMessage"` // Acknowledgment message when request received
}

// RateLimitConfig holds per-user rate limiting settings.
type RateLimitConfig struct {
	Enabled       bool `yaml:"enabled"`       // Enable rate limiting (default false)
	MaxRequests   int  `yaml:"maxRequests"`   // Max requests per window (default 10)
	WindowSeconds int  `yaml:"windowSeconds"` // Window size in seconds (default 60)
}

// BudgetConfig holds per-user spend limits.
type BudgetConfig struct {
	Enabled         bool    `yaml:"enabled"`
	DailyLimitUSD   float64 `yaml:"daily_limit_usd"`
	MonthlyLimitUSD float64 `yaml:"monthly_limit_usd"`
	WarnThreshold   float64 `yaml:"warn_threshold"` // 0.0-1.0, default 0.8
}

// RoleConfig defines a single agent role with routing keywords and tool access.
type RoleConfig struct {
	Name         string   `yaml:"name"`
	Model        string   `yaml:"model"`
	Provider     string   `yaml:"provider"`
	Temperature  float64  `yaml:"temperature"`
	SystemPrompt string   `yaml:"systemPrompt"`
	Keywords     []string `yaml:"keywords"`
	AllowedTools []string `yaml:"allowedTools"`
	Priority     int      `yaml:"priority"`
}

// RTKConfig holds RTK (Rust Token Killer) integration settings.
type RTKConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Commands []string `yaml:"commands"`
}

// Validate applies defaults and enforces constraints on the Config.
func (c *Config) Validate() {
	// Dashboard defaults
	if c.Dashboard.Addr == "" {
		c.Dashboard.Addr = ":8081"
	}

	// Orchestrator defaults and hard cap
	if c.Orchestrator.MaxConcurrent == 0 {
		c.Orchestrator.MaxConcurrent = 5
	}
	if c.Orchestrator.MaxConcurrent > 10 {
		c.Orchestrator.MaxConcurrent = 10
	}

	if c.Orchestrator.SubAgentTimeout.IsZero() {
		c.Orchestrator.SubAgentTimeout = Duration{5 * time.Minute}
	}

	// Session defaults
	if c.Session.MaxHistory == 0 {
		c.Session.MaxHistory = 50
	}

	// Memory defaults
	if c.Memory.MaxArchival == 0 {
		c.Memory.MaxArchival = 10000
	}
	if c.Memory.CoreMemory.MaxEntries == 0 {
		c.Memory.CoreMemory.MaxEntries = 20
	}
	if c.Memory.CoreMemory.MaxValueLen == 0 {
		c.Memory.CoreMemory.MaxValueLen = 500
	}
	if c.Memory.Embedding.Model == "" {
		c.Memory.Embedding.Model = "text-embedding-3-small"
	}

	// Guardrails defaults — enabled by default for safety
	if !c.Security.Guardrails.Input.Enabled && len(c.Security.Guardrails.Input.CustomPatterns) == 0 {
		c.Security.Guardrails.Input.Enabled = true
	}
	if !c.Security.Guardrails.Tool.Enabled && len(c.Security.Guardrails.Tool.RequireApprovalPatterns) == 0 {
		c.Security.Guardrails.Tool.Enabled = true
	}
	if !c.Security.Guardrails.Output.Enabled {
		c.Security.Guardrails.Output.Enabled = true
	}
	// HITL defaults
	if c.Security.HITL.TimeoutMinutes == 0 {
		c.Security.HITL.TimeoutMinutes = 5
	}

	// Whisper defaults
	if c.Whisper.Model == "" {
		c.Whisper.Model = "whisper-large-v3-turbo"
	}
	if c.Whisper.MaxFileSizeMB == 0 {
		c.Whisper.MaxFileSizeMB = 25
	}

	// Skills defaults
	if c.Skills.MarketplaceDir == "" {
		c.Skills.MarketplaceDir = "marketplace"
	}
	if c.Skills.MaxSkillsInPrompt == 0 {
		c.Skills.MaxSkillsInPrompt = 50
	}
	if c.Skills.MaxSkillFileBytes == 0 {
		c.Skills.MaxSkillFileBytes = 262144
	}

	// Budget defaults
	if c.Budget.Enabled && c.Budget.WarnThreshold == 0 {
		c.Budget.WarnThreshold = 0.8
	}

	// Roles defaults — populate 7 default roles if none configured
	if len(c.Roles) == 0 {
		c.Roles = []RoleConfig{
			{Name: "phantom", Model: "", Provider: "", Temperature: 0.7, SystemPrompt: "", Keywords: []string{"restart", "deploy", "server", "status", "docker", "systemctl", "health", "infra", "devops", "service", "nginx", "ssl"}, AllowedTools: nil, Priority: 10},
			{Name: "astrology", Model: "", Provider: "", Temperature: 0.7, SystemPrompt: "", Keywords: []string{"crypto", "bitcoin", "btc", "eth", "ethereum", "trading", "token", "defi", "nft", "wallet", "market", "portfolio", "investment", "stock", "forex", "chart", "candlestick", "pump", "whale"}, AllowedTools: nil, Priority: 20},
			{Name: "wizard", Model: "", Provider: "", Temperature: 0.7, SystemPrompt: "", Keywords: []string{"code", "implement", "function", "bug", "fix", "test", "build", "compile", "git", "deploy", "opencode", "typescript", "golang", "python", "javascript", "refactor", "debug", "api", "endpoint", "database", "sql", "migration"}, AllowedTools: nil, Priority: 30},
			{Name: "artist", Model: "", Provider: "", Temperature: 0.7, SystemPrompt: "", Keywords: []string{"instagram", "tiktok", "twitter", "linkedin", "facebook", "threads", "post", "caption", "hashtag", "reel", "story", "content", "social", "viral", "engagement", "schedule", "publish"}, AllowedTools: nil, Priority: 40},
			{Name: "scribe", Model: "", Provider: "", Temperature: 0.7, SystemPrompt: "", Keywords: []string{"write", "draft", "article", "blog", "email", "document", "copy", "copywriting", "proofread", "translate", "summarize", "report", "newsletter", "pitch", "proposal"}, AllowedTools: nil, Priority: 50},
			{Name: "explorer", Model: "", Provider: "", Temperature: 0.7, SystemPrompt: "", Keywords: []string{"search", "find", "look up", "what is", "explain", "research", "summarize", "web", "browse", "read", "compare", "analyze", "review", "investigate"}, AllowedTools: []string{"memory_search", "web_search", "archival_memory_search", "archival_memory_insert", "core_memory_get"}, Priority: 60},
			{Name: "oracle", Model: "", Provider: "", Temperature: 0.7, SystemPrompt: "", Keywords: nil, AllowedTools: nil, Priority: 100},
		}
	}

	// RTK defaults — populate default commands if enabled but none configured
	if c.RTK.Enabled && len(c.RTK.Commands) == 0 {
		c.RTK.Commands = []string{"cargo", "tsc", "lint", "prettier", "next", "vitest", "playwright", "pnpm", "npm", "npx", "prisma", "docker", "kubectl", "git", "gh", "ls", "grep", "find"}
	}
}
