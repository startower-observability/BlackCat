# BlackCat

A Go-based AI agent that orchestrates [OpenCode CLI](https://opencode.ai) via messaging channels. Deploy BlackCat alongside OpenCode on a server, then interact with your development environment through Telegram, Discord, or WhatsApp.

BlackCat receives your natural language requests, processes them through an LLM-powered agent loop, delegates coding tasks to OpenCode, and responds back in your messaging channel — giving you full server control from anywhere.

## Features

- **Multi-channel messaging** — Telegram, Discord, and WhatsApp adapters
- **8 LLM providers** — OpenAI, Anthropic, GitHub Copilot, Antigravity, Google Gemini, Zen, OpenRouter, Ollama
- **OAuth authentication** — Device code flow (Copilot) and PKCE flow (Antigravity)
- **Zen Coding Plan** — Curated hosted models via OpenCode API
- **Interactive setup** — `blackcat configure` wizard for provider setup
- **OpenCode delegation** — Full access to OpenCode CLI for coding tasks
- **MCP support** — Model Context Protocol server/client integration
- **Encrypted vault** — AES-256-GCM encrypted storage for API keys and tokens
- **Memory consolidation** — Persistent agent memory via MEMORY.md
- **Pixel Cat Dashboard** — React SPA with RPG-style room scene, animated black cat reacting to system state, real-time HUD overlay at `localhost:8081/dashboard/`
- **Security** — Command deny-list, shell sandboxing, auto-permit controls
- **Docker support** — Docker Compose deployment

## Supported Providers

| Provider | Auth Method | Wire Format | Status |
|----------|------------|-------------|--------|
| OpenAI | API Key | OpenAI | Stable |
| Anthropic | API Key | OpenAI-compat | Stable |
| Google Gemini | API Key | Gemini | Stable |
| GitHub Copilot | OAuth Device Flow | OpenAI-compat | New |
| Antigravity | OAuth PKCE | Gemini | New (ToS Risk) |
| OpenRouter | API Key | OpenAI | Stable |
| Ollama | None (local) | OpenAI | Stable |
| Zen Coding Plan | API Key | OpenAI | New |

## Quick Start

### Install

**Linux/macOS** (one-line):
```bash
curl -fsSL https://raw.githubusercontent.com/startower-observability/BlackCat/main/scripts/install.sh | sh
```

**Windows** (PowerShell):
```powershell
irm https://raw.githubusercontent.com/startower-observability/BlackCat/main/scripts/install.ps1 | iex
```

**Or with Go:**
```bash
go install github.com/startower-observability/blackcat@latest
```

### Onboard
```bash
blackcat onboard
```

The wizard guides you through:
1. Choosing an LLM provider
2. Configuring a messaging channel
3. Installing and starting the daemon

### Manage the daemon
```bash
blackcat status     # check status
blackcat restart    # restart after config changes
blackcat stop       # stop the daemon
```

## Deployment

Deploy BlackCat to a Linux VM with a single command:

### Prerequisites

1. Copy the deploy environment template and fill in your VM details:
   ```bash
   cp deploy/deploy.env.example deploy/deploy.env
   $EDITOR deploy/deploy.env
   ```

2. Ensure your SSH key has access to the VM.

### Deploy

```bash
make deploy
```

This single command:
- Pushes your local git changes to the remote
- SSHes into the VM, pulls the latest code, and builds the binary
- Installs the binary to `~/.blackcat/bin/blackcat`
- Deploys and reloads the `blackcat` and `opencode` systemd services
- Runs a health check to confirm the service is up

### Quick Redeploy (skip git push)

```bash
make deploy-no-push
```

### Health Check Only

```bash
make verify
```

See [`deploy/README.md`](deploy/README.md) for full setup instructions including SSH key configuration and service file details.

## Documentation

| Guide | Description |
|-------|-------------|
| [Getting Started](https://startower-observability.github.io/BlackCat/getting-started) | Prerequisites, installation, quick start |
| [Configuration](https://startower-observability.github.io/BlackCat/configuration) | Full YAML reference, environment variables, examples |
| [LLM Providers](https://startower-observability.github.io/BlackCat/providers) | All 8 providers: setup, models, configuration |
| [CLI Reference](https://startower-observability.github.io/BlackCat/cli/onboard) | Reference for all BlackCat commands |
| [Architecture](https://startower-observability.github.io/BlackCat/concepts/architecture) | How BlackCat works internally |

## Configuration

BlackCat is configured via YAML file (`~/.blackcat/config.yaml`) with environment variable overrides using the `BLACKCAT_` prefix.

See [`blackcat.example.yaml`](blackcat.example.yaml) for a complete example with all fields documented.

Key environment variables:

```bash
BLACKCAT_LLM_PROVIDER=openai
BLACKCAT_LLM_APIKEY=sk-your-key
BLACKCAT_CHANNELS_TELEGRAM_TOKEN=your-bot-token
BLACKCAT_VAULT_PASSPHRASE=your-passphrase
BLACKCAT_ZEN_APIKEY=your-zen-key
```

## Docker

```bash
docker compose up -d
```

See `docker-compose.yml` for the full setup. Requires OpenCode CLI to be accessible on the same network.

## Requirements

- Go 1.25+
- OpenCode CLI running on the same server
- At least one messaging channel configured
- At least one LLM provider configured

> **Note:** WhatsApp support requires CGO for SQLite. Build with `CGO_ENABLED=1`.

## License

See [LICENSE](LICENSE) for details.
