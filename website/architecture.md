---
title: Architecture
description: How BlackCat works internally — agent loop, channels, memory, and tool execution
---

# Architecture

BlackCat is a Go-based AI agent that orchestrates OpenCode CLI via messaging channels.

## System Overview

```
┌──────────────────────────────────────────────────────────────┐
│                     BlackCat Agent                          │
│                                                              │
│  ┌────────────┐   ┌────────────┐   ┌────────────────────┐   │
│  │  Telegram   │   │  Discord   │   │     WhatsApp       │   │
│  │  Adapter    │   │  Adapter   │   │     Adapter        │   │
│  └─────┬──────┘   └─────┬──────┘   └──────────┬─────────┘   │
│        │                │                      │             │
│        └────────────────┼──────────────────────┘             │
│                         │                                    │
│                  ┌──────▼──────┐                              │
│                  │ Message Bus │  (fan-in / fan-out)          │
│                  └──────┬──────┘                              │
│                         │                                    │
│                  ┌──────▼──────┐                              │
│                  │ Agent Loop  │  (max 50 turns)              │
│                  └──┬───┬───┬─┘                              │
│                     │   │   │                                │
│           ┌─────────┘   │   └─────────┐                      │
│           │             │             │                      │
│    ┌──────▼─────┐ ┌────▼─────┐ ┌─────▼──────┐               │
│    │ LLM Backend│ │  Tools   │ │  Memory    │               │
│    │  System    │ │ Registry │ │  System    │               │
│    └──────┬─────┘ └────┬─────┘ └────────────┘               │
│           │            │                                     │
│    ┌──────▼─────┐ ┌────▼─────┐                               │
│    │ Provider   │ │ OpenCode │                               │
│    │ Registry   │ │ Delegate │                               │
│    └────────────┘ └──────────┘                               │
│                                                              │
│    ┌────────────┐ ┌────────────┐ ┌────────────┐              │
│    │  Security  │ │  Vault     │ │    MCP     │              │
│    │  Scrubber  │ │ AES-256   │ │ Server/Cli │              │
│    └────────────┘ └────────────┘ └────────────┘              │
└──────────────────────────────────────────────────────────────┘
```

## Core Components

### Agent Loop

**Package:** `agent/` — `loop.go`, `execution.go`, `compaction.go`

The agent loop is the central orchestrator. It receives a user message and iterates up to `maxTurns` (default 50), calling the LLM, executing tool calls, and collecting results until the LLM produces a final text response with no further tool calls.

```
User Message → Build System Prompt → LLM Chat →
  ├─ Text Response → Return to user
  └─ Tool Calls → Execute each tool → Append results → Loop back to LLM
```

Key features:
- **System prompt construction** — Injects workspace context, loaded skills, and memory excerpts
- **Tool execution** — Iterates tool calls from the LLM response, executes via `tools.Registry`, appends results
- **Context compaction** — When conversation grows too long, `compaction.go` summarizes earlier turns to fit within token limits
- **Security scrubbing** — All tool outputs pass through `security.Scrubber` to enforce the command deny-list

### LLM Backend System

**Package:** `llm/` — `backend.go`, `provider.go`, `client.go`, `openai_backend.go`

The `Backend` interface defines the contract all LLM providers must implement:

```go
type Backend interface {
    Chat(ctx context.Context, messages []LLMMessage, tools []ToolDefinition) (*LLMResponse, error)
    Stream(ctx context.Context, messages []LLMMessage, tools []ToolDefinition) (<-chan Chunk, error)
}
```

Supporting types:
- **`BackendConfig`** — Generic config (API key, base URL, model, temperature, max tokens, token source)
- **`BackendFactory`** — Constructor: `func(cfg BackendConfig) (Backend, error)`
- **`BackendInfo`** — Runtime metadata (name, supported models, auth method)
- **`InfoProvider`** — Optional interface for backends to expose `BackendInfo`

The **Backend Registry** (`provider.go`) is a global concurrent-safe map of `BackendFactory` functions keyed by provider name. Providers self-register via `RegisterBackend()`, and the daemon calls `CreateBackend()` at startup to instantiate the configured provider.

### Channel Adapters

**Package:** `channel/` — `channel.go`, plus `telegram/`, `discord/`, `whatsapp/` sub-packages

The `MessageBus` fans-in messages from all registered channel adapters into a single Go channel, and routes outbound responses back to the correct adapter:

- **Telegram** (`channel/telegram/`) — Uses `go-telegram-bot-api/telegram-bot-api/v5`. Long-polling for updates.
- **Discord** (`channel/discord/`) — Uses `bwmarrin/discordgo`. WebSocket gateway connection.
- **WhatsApp** (`channel/whatsapp/`) — Uses `tulir/whatsmeow`. Requires CGO for SQLite (`go build -tags cgo`).

Each adapter implements the `types.Channel` interface:

```go
type Channel interface {
    Start(ctx context.Context, incoming chan<- Message) error
    Send(ctx context.Context, msg Message) error
    Info() ChannelInfo
}
```

### Tools Registry

**Package:** `tools/` — Tool interface with built-in tools and MCP-discovered tools

The `tools.Registry` holds all available tools. Each tool implements `types.Tool`:

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() json.RawMessage  // JSON Schema
    Execute(ctx context.Context, args json.RawMessage) (string, error)
}
```

Built-in tools include shell execution (with security scrubbing), file operations, and OpenCode delegation. MCP-discovered tools are registered dynamically at runtime.

### Memory System

**Package:** `memory/` — `memory.go`

File-based persistent memory using `MEMORY.md`. The `FileStore` appends structured entries and supports automatic consolidation when the entry count exceeds a configurable threshold (default 50). Consolidation summarizes older entries to keep the file manageable while preserving important context.

### Security

**Package:** `security/` — `vault.go`, `scrubber.go`

Two components:
- **Vault** — AES-256-GCM encrypted JSON storage for API keys, OAuth tokens, and secrets. Keyed by string names (e.g., `provider.copilot.oauth`, `llm.apiKey`). Passphrase-derived key via scrypt.
- **Scrubber** — Command deny-list that blocks dangerous shell commands (e.g., `rm -rf /`, `mkfs`, `dd`). Applied to all tool executions. Configurable `autoPermit` mode and custom deny patterns.

### MCP (Model Context Protocol)

**Package:** `mcp/`

Implements both MCP server and client:
- **Server** — Exposes BlackCat's tools over the MCP protocol, allowing external MCP clients to discover and invoke them
- **Client** — Connects to external MCP servers, discovers their tools, and registers them in the `tools.Registry` for the agent loop to use

## Request Lifecycle

A complete request flows through the system as follows:

1. **Channel receives message** — A Telegram/Discord/WhatsApp adapter receives a user message
2. **MessageBus fan-in** — The adapter pushes the message into the shared `incoming` channel
3. **Daemon dispatch** — The daemon's message handler picks up the message and creates a context
4. **Agent loop starts** — `Loop.Run()` begins with the user message, building a system prompt that includes workspace context, skills, and recent memory
5. **LLM call** — The agent calls `Backend.Chat()` (or `Stream()`) with the conversation history and available tool definitions
6. **Tool execution** — If the LLM responds with tool calls, each is executed via `tools.Registry`. Shell commands are scrubbed by `security.Scrubber`. Results are appended to conversation history.
7. **Iteration** — Steps 5-6 repeat until the LLM returns a text response with no tool calls, or `maxTurns` is reached
8. **Response routing** — The final text response is sent back through `MessageBus.Send()` to the originating channel adapter
9. **Memory update** — Key interaction details are appended to `MEMORY.md`

## Configuration Flow

Configuration is loaded with the following precedence (highest wins):

```
Environment Variables  >  YAML Config File  >  Defaults
```

**Package:** `config/` — `config.go`, `loader.go`

1. **Defaults** — Struct field defaults in `config.go` (e.g., temperature 0.7, max tokens 4096)
2. **YAML file** — Loaded from `~/.blackcat/config.yaml` (or path specified by `BLACKCAT_CONFIG`)
3. **Environment variables** — Prefix `BLACKCAT_`, nested with underscores (e.g., `BLACKCAT_LLM_PROVIDER=openai`)

The config struct covers: `ServerConfig`, `OpenCodeConfig`, `LLMConfig`, `ChannelsConfig`, `SecurityConfig`, `MemoryConfig`, `MCPConfig`, `SkillsConfig`, `LoggingConfig`, `OAuthConfig`, `ZenConfig`, and `ProvidersConfig`.

## Provider Architecture

BlackCat supports 8 LLM providers across two wire formats:

| Provider | Package | Wire Format | Auth |
|----------|---------|-------------|------|
| OpenAI | `llm/openai_backend.go` | OpenAI | API key |
| Anthropic | (via OpenAI compat) | OpenAI | API key |
| OpenRouter | (via OpenAI compat) | OpenAI | API key |
| Ollama | (via OpenAI compat) | OpenAI | None |
| GitHub Copilot | `llm/copilot/` | OpenAI | OAuth device flow |
| Zen Coding Plan | `llm/zen/` | OpenAI | API key |
| Google Gemini | `llm/gemini/` | Gemini | API key |
| Antigravity | `llm/antigravity/` | Gemini | OAuth PKCE |

All providers implement `llm.Backend` and register themselves via `llm.RegisterBackend()`.

### Wire Format Codecs

**OpenAI format** — Used by OpenAI, Copilot, Zen, and OpenAI-compatible providers. The `openai_backend.go` wraps `sashabaranov/go-openai` to convert between `types.LLMMessage` and OpenAI's `ChatCompletionMessage`.

**Gemini format** — Used by Google Gemini and Antigravity. The `llm/gemini/codec.go` package defines Gemini-native types (`GeminiRequest`, `GeminiResponse`, `GeminiContent`, `GeminiPart`, `GeminiFuncCall`, `GeminiFuncResp`) and provides `EncodeRequest()` / `DecodeResponse()` functions to convert between BlackCat's `types.LLMMessage` and Gemini wire format.

### OAuth Flow Engine

**Package:** `oauth/` — `device.go`, `pkce.go`, `types.go`, `vault_store.go`

Two OAuth flows for providers that don't use API keys:

**Device Code Flow** (`oauth/device.go`) — Used by GitHub Copilot:
1. Request device code from `github.com/login/device/code`
2. Display user code and verification URL to user
3. Poll `github.com/login/oauth/access_token` until user authorizes
4. Exchange OAuth token for Copilot API token via `api.github.com/copilot_internal/v2/token`
5. Store tokens in vault; Copilot API token refreshes automatically (~30min TTL)

**Browser PKCE Flow** (`oauth/pkce.go`) — Used by Antigravity:
1. Generate PKCE code verifier and challenge (S256)
2. Start local HTTP callback server on ephemeral port
3. Open browser to Google OAuth consent screen with PKCE parameters
4. Receive authorization code via callback
5. Exchange code + verifier for access/refresh tokens
6. Store tokens in vault; supports automatic refresh

**Token Storage** (`oauth/vault_store.go`) — Wraps the security vault to store/retrieve `TokenSet` (JSON-encoded map of token fields) per provider.

## Deployment

BlackCat is designed to run on the **same server** as OpenCode CLI:

```
┌─────────────────────────────────┐
│          Your Server            │
│                                 │
│  ┌─────────────┐ ┌───────────┐  │
│  │ BlackCat  │ │ OpenCode  │  │
│  │   daemon    │◄┤   CLI     │  │
│  └──────┬──────┘ └───────────┘  │
│         │                       │
│    Full server access           │
│    (shell, files, git, etc.)    │
└─────────────────────────────────┘
         │
    Messaging APIs
    (Telegram, Discord, WhatsApp)
```

**Startup:** `blackcat daemon` starts the agent, connects to OpenCode, registers enabled channels, and begins processing messages.

**Docker:** `docker-compose.yml` provides a containerized deployment with both BlackCat and OpenCode in the same compose network.

**Systemd:** Can be deployed as a systemd service for automatic startup and restart on failure.

### Conditional Rules
Rules are `.md` files with YAML frontmatter defining glob patterns:
```yaml
---
name: go-style
globs:
  - "internal/**/*.go"
---
Rule content injected when file matches...
```
Rules inject into `PostFileRead` hook output.

### Session Management
Per-user-per-channel conversation history. Flat-file JSON storage (one file per session). Injected into LLM context up to `max_history` messages. Anonymous users fall back to channel-only key.

### Multi-Agent Orchestration
`Orchestrator.Dispatch()` runs sub-agents in parallel with `errgroup`. Hard cap: 10 concurrent sub-agents. Results returned in original index order. All errors captured per-Result.

### Dashboard
React SPA (PixiJS v8 + React Router v7) embedded in Go binary via `//go:embed`. Token authentication via Bearer header. SPA assets served without auth (login page), API and dashboard pages require auth. Routes:
- `GET /dashboard/login` — Login page (no auth)
- `GET /dashboard/assets/*` — SPA static assets (no auth)
- `GET /dashboard/events` — SSE real-time updates (auth required)
- `GET /dashboard/` and sub-routes — React SPA pages (auth required)
- `GET /dashboard/api/*` — JSON API (auth required)

### Scheduler
Cron-based task scheduling via `robfig/cron/v3`. Uses `SkipIfStillRunning`. Heartbeat task runs every 30s checking subsystem health.

## Phase 3: Subsystem Architecture

BlackCat uses a `Subsystem` interface for pluggable, lifecycle-managed components:

```go
type Subsystem interface {
    Name() string
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Health() SubsystemHealth
}
```

Phase 3 subsystems: Dashboard, Scheduler (with Heartbeat).

### Hook System

Hooks provide lifecycle extension points. Available events:
- `PreChat`, `PostChat` — before/after LLM call
- `PreToolExec`, `PostToolExec` — before/after tool execution
- `PreFileRead`, `PostFileRead` — before/after file read
- `PreFileWrite`, `PostFileWrite` — before/after file write
- `OnSessionStart`, `OnSessionEnd` — session lifecycle

Registration: `registry.Register(hooks.PostFileRead, fn)`
Pre-events short-circuit on error; post-events collect all errors.
Panics are recovered automatically.

### Hierarchical AGENTS.md
Loads AGENTS.md files from workspace root → current directory (max 3 levels). Merged with `\n\n---\n\n` separator. Symlink loop protection via `filepath.EvalSymlinks()`.

### Skills YAML Frontmatter
Skills can define embedded MCP servers in YAML frontmatter:
```yaml
---
name: my-skill
mcpServers:
  - name: server-name
    command: npx
    args: ["-y", "@my/mcp-server"]
---
Skill content here...
```
Files without frontmatter load as plain text (backward compatible).

### Custom Agent Profiles
Profile `.md` files define system prompt overlays. Placed in the profiles directory (configurable). Selected per-request via profile name.
,
## See Also

- [Getting Started](/getting-started) — Installation and setup
- [Configuration Reference](/configuration) — Full config guide
- [LLM Providers](/providers) — Provider details
- [OAuth Setup](/oauth) — Copilot and Antigravity OAuth walkthrough
- [Zen Coding Plan](/zen-plan) — Curated hosted models
- [CLI Configure](/configure-cli) — Interactive setup wizard
