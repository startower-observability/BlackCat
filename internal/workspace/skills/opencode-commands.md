# OpenCode & Oh-My-OpenCode — Referensi Lengkap Semua Command

Kamu BISA dan HARUS menjalankan command `opencode` via tool `exec`.
OpenCode CLI tidak memerlukan input interaktif — semua command bisa dijalankan non-interaktif.

## PENTING: Kamu Punya 2 Cara Menggunakan OpenCode

### Cara 1: Via `exec` Tool (CLI langsung)
Untuk command yang TIDAK perlu session panjang: auth, session list, stats, models, export, dll.
```
exec: opencode auth login
exec: opencode auth list
exec: opencode session list
exec: opencode models
exec: opencode run "analisis file ini" --format json
```

### Cara 2: Via `opencode_task` Tool (Session panjang)
Untuk task coding yang butuh waktu lama: /ulw-loop, /start-work, analisis codebase.
```json
opencode_task: {"prompt": "/ulw-loop\n\nImplementasi fitur auth", "dir": "~/project"}
```

---

## OpenCode CLI Commands

### `opencode run [message..]` — Eksekusi satu kali (non-interaktif)
Mirip `claude code` — jalankan prompt lalu selesai. Cocok untuk analisis, review, task sederhana.
```bash
# Contoh dasar
opencode run "jelaskan arsitektur project ini"

# Dengan model tertentu
opencode run -m claude-sonnet-4.6 "review kode di src/auth.go"

# Dengan agent tertentu
opencode run --agent prometheus "buat rencana kerja untuk fitur X"

# Lanjutkan session sebelumnya
opencode run -c "lanjutkan task sebelumnya"
opencode run -s <session-id> "tambahkan tests"

# Output format JSON (untuk parsing)
opencode run --format json "analisis bug di handler.go"

# Attach file
opencode run -f ./config.yaml "review config ini"

# Jalankan slash command
opencode run --command "/ulw-loop" "implementasi fitur auth"

# Working directory tertentu
opencode run --dir ~/my-project "fix semua lint errors"

# Reasoning effort (variant)
opencode run --variant max "solve complex algorithm problem"
```

**Flags lengkap `opencode run`:**
| Flag | Deskripsi |
|------|-----------|
| `--model, -m` | Pilih model (contoh: claude-sonnet-4.6) |
| `--agent` | Pilih agent (contoh: prometheus, sisyphus) |
| `--continue, -c` | Lanjutkan session terakhir |
| `--session, -s` | Resume session tertentu by ID |
| `--fork` | Fork session (copy context, buat session baru) |
| `--share` | Share session |
| `--format` | Output format: `default` atau `json` |
| `--file, -f` | Attach file (bisa multiple) |
| `--title` | Judul session |
| `--attach` | Connect ke running server (URL) |
| `--dir` | Working directory |
| `--port` | Port server |
| `--variant` | Reasoning effort: high, max, minimal |
| `--thinking` | Tampilkan thinking blocks |
| `--command` | Jalankan slash command |

### `opencode serve` — Headless daemon (HTTP API)
Jalankan OpenCode sebagai background server. Ini yang digunakan `opencode_task`.
```bash
opencode serve --port 4096 --hostname 127.0.0.1
```
| Flag | Deskripsi |
|------|-----------|
| `--port` | Port (default: random) |
| `--hostname` | Hostname (default: 127.0.0.1) |
| `--mdns` | Enable mDNS discovery |
| `--cors` | CORS allowed origins |

### `opencode auth` — Manajemen autentikasi
```bash
opencode auth login              # Login ke semua provider
opencode auth login anthropic    # Login ke provider tertentu
opencode auth logout             # Logout
opencode auth list               # Lihat status auth (alias: ls)
```

### `opencode session` — Manajemen session
```bash
opencode session list            # Lihat semua session
opencode session delete <id>     # Hapus session
```

### `opencode models` — Lihat model tersedia
```bash
opencode models                  # Semua model
opencode models anthropic        # Model dari provider tertentu
```

### `opencode stats` — Statistik penggunaan
```bash
opencode stats                   # Token usage dan cost
```

### `opencode export / import` — Export/import session
```bash
opencode export <session-id>     # Export session ke JSON
opencode import <file-or-url>    # Import session dari JSON/URL
```

### `opencode pr` — GitHub PR
```bash
opencode pr 123                  # Fetch PR #123, checkout, lalu buka opencode
```

### `opencode github` — GitHub integration
```bash
opencode github install          # Install GitHub app
opencode github run              # Run GitHub-triggered task
```

### `opencode attach` — Connect ke server yang sudah jalan
```bash
opencode attach http://localhost:4096
```

### Command lainnya
```bash
opencode upgrade                 # Update opencode
opencode uninstall               # Uninstall
opencode db                      # Database tools
opencode web                     # Web interface
opencode mcp                     # Manage MCP servers
opencode debug                   # Debugging tools
opencode completion              # Shell completion
opencode acp                     # Agent Client Protocol (stdin/stdout)
```

---

## Oh-My-OpenCode — Slash Commands & Agents

### Slash Commands (digunakan dalam prompt `opencode_task` atau `opencode run --command`)

| Command | Deskripsi |
|---------|-----------|
| `/ulw-loop` | Ultra-work loop — jalankan terus sampai semua task selesai |
| `/start-work` | Mulai kerja dari plan Prometheus (harus ada plan dulu) |
| `/handoff` | Buat context summary untuk lanjut di session baru |
| `/ralph-loop` | Self-referential development loop |
| `/init-deep` | Inisialisasi hierarchical AGENTS.md knowledge base |
| `/refactor` | Intelligent refactoring dengan LSP, AST-grep, TDD |
| `/stop-continuation` | Stop semua loop (ralph, ulw, boulder) |
| `/cancel-ralph` | Cancel ralph loop saja |
| `/tokenscope` | Analisis token usage session |
| `/supermemory-init` | Initialize Supermemory |
| `/supermemory-login` | Login ke Supermemory |

### Available Agents

| Agent | Model | Kegunaan |
|-------|-------|----------|
| `sisyphus` | antigravity-opus-4-6-thinking | Worker utama, implementasi code |
| `hephaestus` | gpt-5.3-codex | Worker alternatif, code generation |
| `prometheus` | antigravity-opus-4-6-thinking | Planning — analisis & buat rencana kerja |
| `atlas` | claude-sonnet-4.6 | Master orchestrator, eksekusi work plans |
| `oracle` | gpt-5.2 | Konsultasi read-only, debugging, arsitektur |
| `librarian` | claude-sonnet-4.6 | Research, dokumentasi, cari referensi |
| `explore` | grok-code-fast-1 | Contextual grep, cari pattern di codebase |
| `metis` | claude-sonnet-4.6 | Pre-planning consultant, identifikasi ambiguitas |
| `momus` | gpt-5.2 | Plan reviewer, evaluasi kualitas rencana |
| `multimodal-looker` | gemini-3.1-pro | Analisis gambar/visual |

### Task Categories (untuk delegasi)

| Category | Model | Kegunaan |
|----------|-------|----------|
| `visual-engineering` | gemini-3.1-pro | Frontend, UI/UX, design |
| `ultrabrain` | gpt-5.3-codex | Task logic berat |
| `deep` | gpt-5.3-codex | Problem-solving mendalam |
| `artistry` | gemini-3.1-pro | Pendekatan kreatif non-konvensional |
| `quick` | claude-haiku-4.5 | Task trivial, perubahan kecil |
| `writing` | gemini-3-flash | Dokumentasi, technical writing |
| `git` | gpt-5-mini | Git operations |

---

## Contoh Penggunaan Lengkap

### Setup OpenCode untuk user (via exec)
```bash
# 1. Cek status auth
exec: opencode auth list

# 2. Login (BISA dijalankan via exec — non-interaktif)
exec: opencode auth login

# 3. Cek model tersedia
exec: opencode models

# 4. Test dengan run sederhana
exec: opencode run "hello, test connection"
```

### Analisis codebase (via exec — one-shot)
```bash
exec: opencode run --agent prometheus "analisis arsitektur project ini dan buat work plan" --dir ~/my-project
```

### Implementasi fitur (via opencode_task — session panjang)
```json
// Step 1: Buat plan dengan Prometheus
opencode_task: {"prompt": "Analisis project dan buat rencana untuk implementasi fitur authentication dengan JWT", "dir": "~/my-project"}

// Step 2: Jalankan ULW untuk eksekusi otomatis (lanjutkan session)
opencode_task: {"prompt": "/ulw-loop", "dir": "~/my-project", "session_id": "<session_id dari step 1>"}
```

### Quick code review (via exec)
```bash
exec: opencode run -m claude-sonnet-4.6 "review semua perubahan di git diff HEAD~3" --dir ~/my-project
```
