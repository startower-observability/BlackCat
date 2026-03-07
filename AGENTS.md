# BlackCat Agent Guide

BlackCat is a Go-based AI agent daemon that routes user messages from chat channels (Telegram, Discord, WhatsApp) to specialized AI subagents. This guide covers the internal role system for AI agents working on the codebase.

**Module:** `github.com/startower-observability/blackcat`

## Architecture

```
Message → Daemon → Supervisor → ClassifyMessage → Role → Subagent → LLM
```

1. **Daemon** (`cmd/daemon.go`): Entry point, initializes all components
2. **Supervisor** (`internal/agent/supervisor.go`): Orchestrates message flow
3. **Router** (`internal/agent/router.go`): Classifies messages and assigns roles
4. **Role/Subagent**: Executes the task via delegated LLM calls

## Role System

BlackCat uses a priority-based role router. Each role maps to a subagent with specific capabilities.

| Role | Priority | Keywords | Purpose |
|------|----------|----------|---------|
| `phantom` | 10 | infra, deploy, docker, k8s, server, host, systemd | Infrastructure and DevOps |
| `astrology` | 20 | crypto, blockchain, web3, eth, btc, wallet | Cryptocurrency and Web3 |
| `wizard` | 30 | code, coding, refactor, debug, build, test, go, rust | Software engineering |
| `artist` | 40 | social, post, tweet, thread, image, media | Social media and content |
| `scribe` | 50 | write, doc, readme, blog, article, copy | Writing and documentation |
| `explorer` | 60 | research, search, find, analyze, investigate | Research and information gathering |
| `oracle` | 100 | *fallback* | Default when no other role matches |

Lower priority numbers = higher precedence. The router selects the first matching role in priority order.

## Role Configuration

Roles are defined in `blackcat.yaml` under the `roles:` key:

```yaml
roles:
  - name: wizard
    priority: 30
    description: "Software engineering tasks"
    keywords:
      - code
      - coding
      - refactor
      - debug
    system_prompt: "You are a senior Go engineer..."
    model: gpt-4o
    temperature: 0.2
    max_tokens: 4096
```

### RoleConfig Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Unique role identifier |
| `priority` | int | Lower = higher precedence |
| `description` | string | Human-readable purpose |
| `keywords` | []string | Trigger words for classification |
| `system_prompt` | string | Prompt prepended to each request |
| `model` | string | LLM model override (optional) |
| `temperature` | float | Sampling temperature (optional) |
| `max_tokens` | int | Token limit (optional) |

## Key Go Packages

| Package | Path | Purpose |
|---------|------|---------|
| Config | `internal/config` | YAML/config loading, RoleConfig struct |
| Router | `internal/agent/router.go` | Message classification, role selection |
| Supervisor | `internal/agent/supervisor.go` | Message orchestration, subagent dispatch |
| Daemon | `cmd/daemon.go` | Service entry point |

## RTK (Rust Token Killer)

RTK is a token-saving wrapper for CLI commands. Enable it to reduce LLM context usage:

```bash
# Prefix any command with rtk for condensed output
rtk cargo build
rtk cargo test
rtk git diff
rtk docker ps
```

Use `rtk gain` to see savings stats. BlackCat subagents automatically use RTK where available.

## Development Notes

### Testing

```bash
# Required: use nospa tag to skip SPA compilation in tests
go test -tags nospa ./...
```

### Constraints

- **No new Go dependencies**: Use standard library when possible
- **YAML tags only**: Struct tags must use `yaml:` not `json:`
- **CGO required**: `CGO_ENABLED=1` for WhatsApp SQLite support

### Adding a New Role

1. Define in `blackcat.yaml` with unique name and priority
2. Add keywords that trigger the role
3. Set appropriate system_prompt for the subagent persona
4. Test classification: `go test -tags nospa ./internal/agent/...`

### File Locations

```
internal/
  config/
    config.go          # Config and RoleConfig structs
  agent/
    router.go          # ClassifyMessage, role selection
    supervisor.go      # Message orchestration
    subagent.go        # Subagent execution
  roles/
    *.yaml             # Role definitions (optional dir)
cmd/
  daemon.go            # Daemon entry point
```
