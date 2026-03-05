---
name: OpenCode ULW Workflow
tags: [opencode, ultrawork, ulw, workflow]
---

# OpenCode ULW Workflow

Gunakan workflow Ultra-Work Loop (ULW) untuk menangani tugas coding yang kompleks secara otonom tanpa perlu interaksi per langkah. Cocok untuk investigasi, perbaikan bug, atau implementasi fitur sedang yang membutuhkan iterasi mandiri.

## Kapan Menggunakan ULW
- Tugas coding multi-file yang membutuhkan siklus "pikir-buat-tes".
- Implementasi fitur baru atau refactoring yang sudah memiliki gambaran teknis.
- Perbaikan bug yang membutuhkan eksplorasi codebase dan verifikasi tes.

## Struktur Panggilan opencode_task
Panggil tool `opencode_task` dengan menyertakan command `/ulw-loop` di awal prompt.

```json
{
  "prompt": "/ulw-loop\n\n[Deskripsi tugas detail di sini]",
  "dir": "D:/Projects/StarTower/interstellar",
  "session_id": "optional-id-jika-melanjutkan"
}
```

## Contoh Template Prompt
### 1. Eksekusi Langsung (Tugas Baru)
```text
/ulw-loop

Implementasikan middleware autentikasi JWT di folder internal/auth. 
Pastikan ada unit test dan integrasikan ke router utama di main.go.
```

### 2. Rencana Lalu Eksekusi (Direkomendasikan untuk Tugas Kompleks)
**Langkah 1: Minta Perencanaan (Prometheus)**
`opencode_task(prompt="Analisis struktur database saat ini dan buat rencana migrasi ke PostgreSQL.", dir="...")`

**Langkah 2: Jalankan ULW dengan Session ID dari Langkah 1**
`opencode_task(prompt="/ulw-loop", dir="...", session_id="SESSION_DARI_LANGKAH_1")`

## Langkah Kerja (Step-by-Step)
1. **Analisis**: Jika tugas sangat besar, biarkan Prometheus membuat rencana terlebih dahulu dalam sesi biasa.
2. **Inisiasi**: Panggil `/ulw-loop` bersama deskripsi tugas atau dalam sesi yang sudah memiliki rencana.
3. **Otonomi**: OpenCode akan berjalan terus menerus (loop) sampai semua sub-task selesai.
4. **Verifikasi**: Periksa hasil akhir (build/test) sebelum melaporkan selesai ke user.

## Kesalahan Umum
- ❌ Mengirim deskripsi tanpa `/ulw-loop` di baris pertama: OpenCode hanya akan merespon teks biasa tanpa menjalankan loop otonom.
- ❌ Tidak menyertakan `session_id` saat ingin melanjutkan pekerjaan: OpenCode akan memulai sesi baru dan kehilangan konteks file yang sudah dibaca/diubah.
- ❌ Mengabaikan direktori (`dir`): OpenCode mungkin bekerja di folder yang salah atau gagal menemukan repo git.
