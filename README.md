> [!TIP]
> For AI agents setting up this project, skip this README and read [`llms.txt`](./llms.txt) instead.

# BlackCat 🐈‍⬛

**A black cat sorcerer that bridges your messaging channels to a full AI coding environment.**

BlackCat is a Go daemon that sits between your chat apps (Telegram, Discord, WhatsApp) and [OpenCode CLI](https://opencode.ai). Send a message, and your digital familiar conjures code changes, runs commands, and reports back — all from your phone.

Once summoned, the sorcery is autonomous: BlackCat handles LLM orchestration, tool delegation, encrypted secret storage, scheduled tasks, and a pixel-art dashboard where a cat reacts to your system state in real-time.

## Highlights

| | Feature | Description |
|---|---------|-------------|
| 💬 | **Multi-Channel** | Telegram, Discord, and WhatsApp adapters — chat from anywhere |
| 🧠 | **8 LLM Providers** | OpenAI, Anthropic, Gemini, Copilot, Antigravity, Zen, OpenRouter, Ollama |
| 🔐 | **OAuth + Vault** | Device code flow, PKCE, AES-256-GCM encrypted key storage |
| 🐱 | **Pixel Cat Dashboard** | RPG-style room scene with animated black cat at `localhost:8081/dashboard/` |
| ⏰ | **Scheduler** | 6-field cron jobs that deliver messages to channels on schedule |
| 🧰 | **OpenCode Delegation** | Full access to OpenCode CLI for coding, debugging, refactoring |
| 🔌 | **MCP Support** | Model Context Protocol server/client integration |
| 🧹 | **Memory** | Persistent agent memory via MEMORY.md with auto-consolidation |

## Supported Providers

| Provider | Auth | Wire Format | Status |
|----------|------|-------------|--------|
| OpenAI | API Key | OpenAI | Stable |
| Anthropic | API Key | OpenAI-compat | Stable |
| Google Gemini | API Key | Gemini | Stable |
| GitHub Copilot | OAuth Device Flow | OpenAI-compat | Stable |
| Antigravity | OAuth PKCE | Gemini | New (ToS Risk) |
| OpenRouter | API Key | OpenAI | Stable |
| Ollama | None (local) | OpenAI | Stable |
| Zen Coding Plan | API Key | OpenAI | Stable |

## Installation

### For Humans

```bash
go install github.com/startower-observability/blackcat@latest
blackcat onboard
```

The `onboard` wizard walks you through:
1. Picking an LLM provider and entering credentials
2. Connecting a messaging channel
3. Installing and starting the daemon

That's it. You're live.

### For AI Agents

Point your LLM at [`llms.txt`](./llms.txt) — it contains the full deterministic setup contract.

## Core Commands

```bash
blackcat onboard            # first-time setup wizard
blackcat configure          # reconfigure provider/channel anytime
blackcat start              # start the daemon
blackcat stop               # stop the daemon
blackcat restart            # restart after config changes
blackcat status             # check daemon state
blackcat health             # quick health check (JSON)
blackcat doctor             # full system diagnostic
blackcat channels list      # list configured channels
blackcat channels login     # authenticate a channel session
```

## Configuration

Config file: `~/.blackcat/config.yaml` (created by `blackcat onboard`)

Environment variable overrides use the `BLACKCAT_` prefix:

```bash
BLACKCAT_LLM_PROVIDER=openai
BLACKCAT_LLM_APIKEY=sk-your-key
BLACKCAT_CHANNELS_TELEGRAM_TOKEN=your-bot-token
BLACKCAT_CHANNELS_DISCORD_TOKEN=your-discord-token
BLACKCAT_VAULT_PASSPHRASE=your-passphrase
BLACKCAT_ZEN_APIKEY=your-zen-key
BLACKCAT_OPENCODE_PASSWORD=your-opencode-password
```

See [`blackcat.example.yaml`](blackcat.example.yaml) for the full reference.

## Requirements

- Go 1.25+ with `CGO_ENABLED=1` (required for WhatsApp SQLite support)
- [OpenCode CLI](https://opencode.ai) running on the same machine
- At least one messaging channel configured
- At least one LLM provider API key

## License

See [LICENSE](LICENSE) for details.
