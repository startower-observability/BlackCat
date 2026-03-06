---
name: OpenCode Start Work Runbook
tags: [opencode, plan, start-work, atlas]
---

# OpenCode Start Work Runbook

Workflow dua tahap: Prometheus buat rencana, lalu /start-work eksekusi rencana tersebut. Cocok untuk proyek besar lintas modul.

## Kapan Menggunakan
- Implementasi fitur besar lintas banyak file
- Migrasi arsitektur atau refactoring besar
- Task yang butuh pelacakan progres per checklist

## Cara Menjalankan

### Langkah 1: Planning dengan Prometheus
```
exec: opencode run --dir <PATH_PROJECT> --agent prometheus "analisis dan buat rencana kerja untuk [deskripsi task]"
```

### Langkah 2: Execute dengan /start-work
Lanjutkan session yang sama:
```
exec: opencode run --dir <PATH_PROJECT> -c --command "/start-work"
```

## Contoh Lengkap
```
exec: opencode run --dir /home/gillv/projects/myapp --agent prometheus "buat rencana untuk menambah integrasi Prometheus metrics di /internal/observability, expose endpoint /metrics, instrumentasi gRPC middleware"
```
Setelah planning selesai:
```
exec: opencode run --dir /home/gillv/projects/myapp -c --command "/start-work"
```

## Fallback via opencode_task
```
opencode_task: {"prompt": "@prometheus Buat rencana migrasi ke Redis", "dir": "/path/project"}
```
Lalu:
```
opencode_task: {"prompt": "/start-work", "dir": "/path/project", "session_id": "SESSION_DARI_LANGKAH_1"}
```

## PENTING
- SELALU jalankan Prometheus planning SEBELUM /start-work
- Gunakan -c untuk melanjutkan session planning ke execution
- SELALU tentukan --dir yang benar
- Simpan path project ke core_memory

## Kesalahan Umum
- Jalankan /start-work tanpa planning dulu: tidak ada rencana untuk dieksekusi
- Tidak lanjutkan session yang sama: agent baru tidak tahu apa yang direncanakan
