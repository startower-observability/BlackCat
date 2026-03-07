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
| 🎙️ | **Voice Transcription** | Automatic voice-to-text via Groq Whisper for Telegram, Discord, WhatsApp |
| 📱 | **Social Media Skills** | Built-in skills for Threads, Twitter/X, LinkedIn, Facebook, TikTok, Google Workspace |
| 🎭 | **Role-Based Routing** | 7 configurable agent personas route messages by keyword matching |
| ⚡ | **RTK Integration** | Optional token-saving wrapper for shell commands |

## Role System

BlackCat routes messages to specialized agent personas based on keyword matching. Each role has a priority level; higher priority roles are checked first. Messages matching a role's keywords are handled by that persona's system prompt and capabilities.

| Role | Priority | Example Keywords | Description |
|------|----------|------------------|-------------|
| phantom | 10 | deploy, build, docker, k8s, helm, terraform | Infrastructure and DevOps automation |
| astrology | 20 | crypto, token, price, chart, trading, eth, btc | Cryptocurrency and trading analysis |
| wizard | 30 | code, refactor, debug, test, function, class | Coding and software development |
| artist | 40 | post, tweet, thread, image, design, caption | Social media and creative content |
| scribe | 50 | write, summarize, edit, draft, proofread | Writing and documentation tasks |
| explorer | 60 | search, find, research, lookup, who, what | Research and information retrieval |
| oracle | 100 | (fallback) | General-purpose fallback for unmatched messages |

Roles are fully customizable via `blackcat.yaml`. Add, remove, or modify roles to match your workflow.

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
BLACKCAT_WHISPER_GROQAPIKEY=your-groq-api-key
```

See [`blackcat.example.yaml`](blackcat.example.yaml) for the full reference.

## Built-in Skills

| Skill | Requires | Auth |
|-------|----------|------|
| Threads | `THREADS_ACCESS_TOKEN` | Meta Graph API token |
| Twitter/X | `bird` CLI + `TWITTER_AUTH_TOKEN` | Browser cookie |
| LinkedIn | `python3` + `linkedin-api` + `LINKEDIN_LI_AT` + `LINKEDIN_JSESSIONID` | Browser cookies |
| Facebook | `FACEBOOK_PAGE_TOKEN` | Meta Graph API token |
| TikTok | `TIKTOK_ACCESS_TOKEN` | TikTok Content API token |
| Google Workspace | `gws` CLI (Node 18+) | `gws auth setup` |

### Phase 2 Skills

| Skill | Requires | Description |
|-------|----------|-------------|
| `veo3-video-gen` | `uv`, `ffmpeg`, `GEMINI_API_KEY` | Generate videos with Google Veo 3 |
| `nano-banana-pro` | `uv`, `GEMINI_API_KEY` | Generate/edit images with Gemini |
| `document-processing` | `python3` | Extract text from PDF/DOCX/XLSX/PPTX |
| `capability-evolver` | `node` | Self-evolution: propose and register new skills |
| `reddit-scraper` | `python3` | Scrape Reddit posts/comments via public JSON |
| `prompt-guard` | `python3` | Detect and neutralize prompt injection attacks |
| `marketplace-installer` | `npx` | Install/manage marketplace skills |

Skills are silently skipped when prerequisites are not met. Run `blackcat doctor` to check.

## 🛒 Skills Marketplace

The marketplace lets you install community skills into `~/.blackcat/marketplace/`. Skills in the marketplace directory are automatically loaded alongside built-in skills.

### Install a marketplace skill

```bash
npx blackcat-skill install <skill-name>
```

### Marketplace directory structure

```
~/.blackcat/
  skills/         # built-in skills (managed by blackcat)
  marketplace/    # community skills (managed by you)
```

### Requirements

- `npx` — for installing marketplace skills
- `allow_external_install: true` in config (disabled by default for security)

## Requirements

- Go 1.25+ with `CGO_ENABLED=1` (required for WhatsApp SQLite support)
- [OpenCode CLI](https://opencode.ai) running on the same machine
- At least one messaging channel configured
- At least one LLM provider API key
- Node.js 18+ (optional — required for Google Workspace CLI skill)
- Python 3 (optional — required for LinkedIn skill)

## Phase 3 Features

### Provider Fallback

Configure backup LLM providers that are tried in order if the primary fails:

```yaml
llm:
  provider: openai
  fallback:
    - copilot
    - gemini
```

Valid fallback names: `openai`, `copilot`, `antigravity`, `gemini`, `zen`.

### Budget Controls

Set daily and monthly spend limits to prevent runaway API costs:

```yaml
budget:
  enabled: true
  daily_limit_usd: 10.00
  monthly_limit_usd: 100.00
  warn_threshold: 0.8
```

When a limit is exceeded, the daemon rejects the request and informs the user. At the warn threshold, a warning is logged and processing continues.

### Skill Versioning and Dependencies

Skills can declare a semantic version and depend on other skills:

```yaml
# In skill frontmatter
version: "v1.2.0"
depends_on:
  - coding
  - research
```

Skills are loaded in dependency order (topological sort). Circular dependencies are detected and rejected. Missing dependencies are logged and the dependent skill is removed.

### Flexible Prerequisites (anyBins)

Skills can declare flexible binary requirements using OR logic within groups:

```yaml
requires:
  any_bins:
    - [python3, python]   # either python3 or python satisfies this group
    - [node, nodejs]      # AND either node or nodejs
```

### Marketplace Registry

A `registry.json` file in the marketplace directory describes available skills with version and install metadata. Run `blackcat doctor` to check registry status and skill install hints.

## License

See [LICENSE](LICENSE) for details.
