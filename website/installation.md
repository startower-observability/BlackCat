---
title: Installation
description: Detailed installation guide for BlackCat across different platforms
---

# Installation

Choose the installation method that best fits your environment.

## One-line Installer

The easiest way to install BlackCat is via our shell script.

### Linux & macOS

```bash
curl -fsSL https://raw.githubusercontent.com/startower-observability/BlackCat/main/scripts/install.sh | sh
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/startower-observability/BlackCat/main/scripts/install.ps1 | iex
```

## Go Install

If you have Go installed (1.25 or later), you can install the binary directly from the source:

```bash
go install github.com/startower-observability/blackcat@latest
```

## Build from Source

For a manual build with all features (including the web dashboard):

```bash
git clone https://github.com/startower-observability/blackcat.git
cd blackcat
make build-all   # builds React SPA + embeds into Go binary
```

Or build manually step by step:

```bash
cd web && npm ci && npm run build   # Build React SPA
cd .. && CGO_ENABLED=1 go build -tags fts5 -o blackcat .
```

### WhatsApp Support (CGO)

WhatsApp requires CGO for SQLite. The `make build-all` command already includes CGO. For a minimal build without WhatsApp:

```bash
go build -o blackcat .
```

## Updating

To update to the latest version, re-run the one-line installer or the `go install` command:

```bash
go install github.com/startower-observability/blackcat@latest
```

## Uninstalling

To remove the BlackCat binary, daemon service, and configuration files:

```bash
blackcat uninstall --yes
rm -rf ~/.blackcat
```
