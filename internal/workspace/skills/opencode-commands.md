# OpenCode — Referensi Command untuk Agent

Kamu BISA dan HARUS menjalankan command `opencode` via tool `exec`.

## Cara Utama: `opencode run` via exec

Gunakan `exec` untuk SEMUA interaksi dengan OpenCode. Ini lebih reliable daripada `opencode_task`.

```
exec: opencode run --dir <path> "prompt atau instruksi"
```

### ⚠️ WAJIB: Tentukan `--dir` yang Benar!
`--dir` HARUS diisi dengan path project yang tepat. Tanpa `--dir`, OpenCode jalan di directory yang salah.

Cara menentukan --dir secara dinamis:
1. Jika user baru clone repo, gunakan path hasil clone
2. Jika user sebut nama project, cari dulu: exec: find ~ -maxdepth 3 -name '.git' -type d 2>/dev/null
3. Cek core_memory untuk path yang pernah disimpan
4. Jika ragu, TANYA user

PENTING: Setelah menemukan path project, SELALU simpan ke core_memory agar tidak lupa:
exec: core_memory_update("project_paths", "blackcat: /home/gillv/projects/blackcat")

---

## Command Reference

### opencode run — Eksekusi satu kali
```
exec: opencode run --dir ~/project "analisis arsitektur project ini"
exec: opencode run --dir ~/project --agent atlas "review code quality"
exec: opencode run --dir ~/project --agent prometheus "buat rencana kerja"
exec: opencode run --dir ~/project -m claude-sonnet-4.6 "review perubahan terbaru"
```

Flags penting:
--dir PATH          Working directory (WAJIB)
--agent NAME        Pilih agent: atlas, prometheus, sisyphus, oracle, librarian
-m MODEL            Pilih model: claude-sonnet-4.6, gpt-5.2, dll
-c                  Lanjutkan session terakhir
-s SESSION_ID       Resume session tertentu
--format json       Output JSON (untuk parsing)
-f FILE             Attach file
--variant LEVEL     Reasoning effort: high, max, minimal
--command CMD       Jalankan slash command

### Melanjutkan Session
```
exec: opencode run --dir ~/project -c "lanjutkan task sebelumnya"
exec: opencode run --dir ~/project -s ses_abc123 "tambahkan tests"
```

### Slash Commands via CLI
```
exec: opencode run --dir ~/project --command "/ulw-loop" "implementasi fitur auth"
exec: opencode run --dir ~/project --command "/start-work"
exec: opencode run --dir ~/project --command "/handoff"
```

### Auth dan Setup
```
exec: opencode auth list
exec: opencode auth login
exec: opencode auth login anthropic
```

### Info dan Status
```
exec: opencode session list
exec: opencode models
exec: opencode stats
```

---

## Agents yang Tersedia

prometheus — Planning agent, analisis dan buat rencana kerja
atlas — Master orchestrator, eksekusi work plans
sisyphus — Worker utama, implementasi code
oracle — Konsultasi read-only, debugging, arsitektur
librarian — Research, dokumentasi, cari referensi
explore — Contextual grep, cari pattern di codebase
hephaestus — Worker alternatif
metis — Pre-planning consultant
momus — Plan reviewer

---

## Slash Commands

/ulw-loop — Ultra-work loop, jalankan sampai semua task selesai
/start-work — Mulai kerja dari plan Prometheus
/handoff — Buat context summary untuk lanjut di session baru
/ralph-loop — Self-referential development loop
/init-deep — Inisialisasi AGENTS.md knowledge base
/refactor — Intelligent refactoring
/stop-continuation — Stop semua loop

---

## Workflow Contoh

### Analisis codebase
exec: opencode run --dir ~/projects/blackcat --agent atlas "analisa keseluruhan project, sebutkan improvement code quality, maintainability, dan security"

### ULW untuk implementasi
exec: opencode run --dir ~/projects/blackcat --command "/ulw-loop" "implementasi fitur authentication JWT di internal/auth"

### Plan lalu execute
Langkah 1: exec: opencode run --dir ~/projects/myapp --agent prometheus "buat rencana migrasi database ke PostgreSQL"
Langkah 2: exec: opencode run --dir ~/projects/myapp -c --command "/start-work"

### Lanjutkan session
exec: opencode session list
exec: opencode run --dir ~/projects/myapp -s ses_xxx "lanjutkan implementasi"

---

## Fallback: opencode_task Tool

Jika exec timeout (task sangat lama >600s), gunakan opencode_task sebagai fallback:
opencode_task: {"prompt": "/ulw-loop\n\nDeskripsi task", "dir": "/path/ke/project"}

opencode_task cocok untuk task yang butuh lebih dari 10 menit.

---

## Long-Running Tasks dengan opencode_task_async

Gunakan `opencode_task_async` untuk task yang sangat lama (>10 menit) atau background processing.

### Perbedaan: opencode_task vs opencode_task_async

| Tool | Cara Kerja | Gunakan Saat |
|------|------------|--------------|
| opencode_task | Blocking, tunggu sampai selesai | Task < 10 menit, butuh hasil langsung |
| opencode_task_async | Non-blocking, return task ID | Task > 10 menit, background processing |

### Cara Pakai opencode_task_async

```
opencode_task_async: {"prompt": "Refactor semua service layer", "dir": "/home/user/project", "recipient_id": "+628123456789"}
```

Parameter:
- prompt (WAJIB): Deskripsi task
- dir (WAJIB): Path project absolute
- recipient_id (optional): Nomor WhatsApp untuk notifikasi saat selesai (format: +628xxx)

### Pattern yang Benar

**Langkah 1: Start task async**
```
Task dimulai dengan ID: 42. Cek status dengan check_opencode_status.
```

**Langkah 2: Cek status (WAJIB sebelum claim apa pun)**
```
check_opencode_status: {"session_id": "ses_xxx"}
```

**Langkah 3: Report berdasarkan hasil check**
```
Sesi masih aktif, task sedang berjalan.
atau
Task selesai. Hasil: ...
```

### Notification Otomatis dengan recipient_id

Set `recipient_id` ke nomor WhatsApp user, mereka akan dapat notifikasi otomatis saat task selesai:

```
User request task lama. Response:
"Task dimulai (ID: 42). Kamu akan dapat notifikasi WhatsApp saat selesai."
```

### Contoh Lengkap Workflow

```
User: "Refactor seluruh codebase ke clean architecture"

Agent:
1. opencode_task_async: {"prompt": "Refactor ke clean architecture...", "dir": "/project", "recipient_id": "+628xxx"}
2. Response: "Task refactoring dimulai (ID: 42). Cek status kapan saja atau tunggu notifikasi WhatsApp."

(Jika user tanya status nanti)
3. check_opencode_status: {"session_id": "ses_xxx"}
4. Response berdasarkan hasil check
```

### Anti-Pattern: JANGAN LAKUKAN INI

```
❌ JANGAN claim "task masih running" tanpa check_opencode_status dulu
❌ JANGAN bilang "sepertinya masih jalan" tanpa evidence
❌ JANGAN asumsi status task tanpa verify
```

**WAJIB selalu call `check_opencode_status` sebelum claim status apa pun.**

