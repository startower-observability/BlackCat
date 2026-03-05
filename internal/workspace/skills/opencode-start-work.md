---
name: OpenCode Start Work Runbook
tags: [opencode, plan, start-work, atlas]
---

# OpenCode Start Work Runbook

Workflow ini digunakan untuk eksekusi terstruktur dari rencana kerja (Work Plan) yang telah disusun oleh Prometheus. Cocok untuk proyek besar yang melibatkan banyak file, dependensi, dan langkah verifikasi bertahap.

## Kapan Menggunakan Start-Work
- Implementasi fitur besar lintas modul/folder.
- Migrasi arsitektur atau refactoring besar-besaran.
- Tugas yang membutuhkan pelacakan progres per item checklist.

## Struktur Panggilan opencode_task
Panggilan ini selalu merupakan proses dua tahap (Two-Stage Workflow).

### Langkah 1: Perencanaan dengan Prometheus
Gunakan `@prometheus` (atau planning agent) untuk membuat file rencana `.sisyphus/plans/*.md`.

```json
{
  "prompt": "@prometheus Analisis sistem caching saat ini dan buat rencana migrasi ke Redis.",
  "dir": "D:/Projects/StarTower/interstellar"
}
```

### Langkah 2: Eksekusi dengan /start-work
Panggil `/start-work` dalam sesi yang sama atau dengan `session_id` dari Langkah 1.

```json
{
  "prompt": "/start-work",
  "dir": "D:/Projects/StarTower/interstellar",
  "session_id": "SESSION_DARI_LANGKAH_1"
}
```

## Alur Kerja (Workflow)
1. **Prometheus Planning**: `@prometheus [deskripsi tugas]`. Pastikan outputnya menghasilkan rencana kerja yang valid.
2. **Execute**: Jalankan `/start-work`. Atlas akan membaca rencana kerja dan mengeksekusi tiap item.
3. **Monitor**: Amati log progres untuk melihat checklist mana yang sedang dikerjakan.
4. **Finalize**: Atlas akan berhenti jika rencana selesai atau terjadi kesalahan fatal. Lakukan verifikasi manual (build/test) sebelum lapor.

## Contoh Template Prompt
### 1. Planning Tugas Infrastruktur
```text
@prometheus Saya ingin menambah integrasi Prometheus metrics di folder /internal/observability. 
Buat rencana detail untuk expose endpoint /metrics dan instrumentasi gRPC middleware.
```

### 2. Eksekusi Rencana
```text
/start-work
```

## Kesalahan Umum
- ❌ Menjalankan `/start-work` tanpa sesi perencanaan (`@prometheus`) sebelumnya: OpenCode tidak akan menemukan file rencana untuk dieksekusi.
- ❌ Tidak menyertakan `session_id` saat beralih dari fase planning ke fase execution: Agent baru tidak akan tahu apa yang direncanakan oleh agent sebelumnya.
- ❌ Mengedit file rencana secara manual di tengah jalan: Dapat merusak status checklist dan alur Atlas.
