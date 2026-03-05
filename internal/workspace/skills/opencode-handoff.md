---
name: OpenCode Handoff Quality
tags: [opencode, handoff, continuity, context]
---

# OpenCode Handoff Quality

Workflow ini digunakan untuk memindahkan tugas (handoff) ke sesi OpenCode baru tanpa kehilangan konteks kerja penting. Sangat krusial saat context window mulai penuh atau saat ingin melanjutkan pekerjaan di lain waktu.

## Kapan Wajib Handoff
- Sesi saat ini sudah terlalu panjang dan respons mulai melambat.
- Pekerjaan belum selesai dan perlu dilanjutkan dalam sesi baru yang bersih.
- Mentransfer status kerja dari sesi planning ke sesi eksekusi terpisah.

## Struktur Panggilan opencode_task
Panggil command `/handoff` dalam sesi OpenCode yang sedang berjalan.

```json
{
  "prompt": "/handoff",
  "dir": "D:/Projects/StarTower/interstellar",
  "session_id": "SESI_YANG_SEDANG_BERJALAN"
}
```

## Langkah Kerja (Step-by-Step)
1. **Initiate**: Panggil `/handoff` di sesi lama.
2. **Result**: OpenCode akan menghasilkan ringkasan (Summary) dari status kerja terakhir.
3. **Capture**: Salin ringkasan tersebut (Goal, Progress, Files touched, Pending tasks).
4. **Resume**: Mulai sesi baru (`opencode_task`) dengan menempelkan ringkasan handoff sebagai prompt awal di `session_id` yang baru atau sesi baru.

## Contoh Template Prompt
### 1. Inisiasi Handoff
```text
/handoff
```

### 2. Melanjutkan di Sesi Baru (Handoff Summary)
```text
Resume tugas: Implementasi middleware autentikasi JWT.
- Goal: Menambah proteksi di folder /api/v1.
- Status: Middleware selesai, butuh integrasi ke router.
- Files: internal/auth/middleware.go (sudah dibuat).
- Next: Update main.go untuk mengaktifkan middleware.
```

## Isi Minimal Summary Handoff
Ringkasan handoff yang baik harus mencakup:
- **Task Goal**: Apa tujuan akhir tugas ini?
- **Current Status**: Apa yang sedang dikerjakan sekarang?
- **Completed**: Langkah mana yang sudah benar-benar selesai?
- **Pending**: Apa langkah spesifik berikutnya?
- **Context Files**: File mana yang baru saja diubah atau dibaca?

## Kesalahan Umum
- ❌ Hanya mengetik `/handoff` tanpa mengolah hasil ringkasannya ke sesi baru: Ringkasan tersebut akan hilang percuma.
- ❌ Handoff di tengah proses kompilasi atau tes yang sedang berjalan: Dapat menyebabkan status kerja tidak sinkron.
- ❌ Tidak menyertakan constraint penting dalam ringkasan: Sesi baru mungkin mengulangi kesalahan yang sama atau melanggar aturan arsitektur.
