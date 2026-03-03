---
title: Dashboard
description: Real-time pixel cat dashboard for monitoring your BlackCat deployment
---

# Dashboard Guide

The BlackCat Dashboard is a React SPA featuring a **top-down RPG-style room scene** with an animated black cat character that reacts to system state in real time. Subsystem health, agent status, and live events are displayed as RPG-styled HUD overlay panels.

## Setup

Enable the dashboard in your `config.yaml`:

```yaml
dashboard:
  enabled: true
  addr: ":8081"
  token: "your-secret-auth-token"
```

## Accessing the Dashboard

1. Start the daemon: `blackcat daemon`
2. Open `http://localhost:8081/dashboard/` in your browser
3. Log in with your configured `token`

The login page loads without authentication — only the dashboard pages and API require a valid token.

## Pages

| Page | URL | Description |
|------|-----|-------------|
| **Home** | `/dashboard/` | Room scene with pixel cat + live HUD overlay |
| **Agents** | `/dashboard/agents` | Health cards for all registered subsystems |
| **Tasks** | `/dashboard/tasks` | Scheduled jobs and their current status |
| **Schedule** | `/dashboard/schedule` | Cron schedule overview |

## Cat Animation States

The black cat character animates based on system state:

| State | Trigger | Animation |
|-------|---------|-----------|
| `idle` | All subsystems healthy, no active OpenCode session | Gentle idle loop |
| `working` | OpenCode subsystem active/processing | Busy working animation |
| `error` | Any subsystem failed, degraded, or stopped | Error animation |
| `thinking` | Client-side: waiting for LLM response | Thinking bubble |
| `success` | Client-side: task completed (5s transient) | Celebration animation |

Query the current state via the API:
```bash
curl -H "Authorization: Bearer <token>" http://localhost:8081/dashboard/api/cat-state
# → {"state":"idle","description":"All systems nominal","since":"..."}
```

## Real-Time Updates (SSE)

Live updates are pushed via Server-Sent Events at `/dashboard/events`. The React frontend subscribes automatically and updates the HUD panels and cat animation without page refresh.

Event types:
- `heartbeat` — 30s keepalive; triggers status refresh
- `agent-update` — subsystem state changed; triggers agent cards refresh
- `success` — task completed; triggers 5s success animation on cat

## API Reference

All API endpoints require `Authorization: Bearer <token>` header.

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/dashboard/api/cat-state` | GET | Current cat animation state |
| `/dashboard/api/status` | GET | All subsystem health |
| `/dashboard/api/me` | GET | Authentication check |
| `/dashboard/api/agents` | GET | Agent health details |
| `/dashboard/api/health` | GET | System health summary |

## Building from Source

The dashboard is a React SPA embedded in the Go binary. To build:

```bash
# Install Node.js dependencies and build SPA + binary together
make build-all

# Or step by step:
cd web && npm ci && npm run build   # Build React SPA → internal/dashboard/dist/
cd .. && CGO_ENABLED=1 go build -tags fts5 -o blackcat .   # Embed SPA into binary
```

## Development Mode

Run the Vite dev server with API proxy for fast iteration:

```bash
make dev-web   # Starts Vite at http://localhost:5173/dashboard/
               # Proxies /dashboard/api/* → :8081
```

## Configuration Reference

| Field | YAML key | Default | Description |
|-------|----------|---------|-------------|
| Enabled | `enabled` | `false` | Enable web dashboard |
| Addr | `addr` | `":8081"` | Listen address |
| Token | `token` | `""` | Bearer auth token |
