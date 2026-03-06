---
name: OpenCode Handoff
tags: [opencode, handoff, continuity, context]
---

# OpenCode Handoff

Pindahkan task ke session baru tanpa kehilangan konteks. Gunakan saat session terlalu panjang atau mau lanjutkan nanti.

## Kapan Menggunakan
- Session sudah panjang dan respons melambat
- Pekerjaan belum selesai, perlu dilanjutkan nanti
- Transfer dari planning ke execution di session terpisah

## Cara Menjalankan

### Via exec (lanjutkan session terakhir)
```
exec: opencode run --dir <PATH_PROJECT> -c --command "/handoff"
```

### Via exec (session tertentu)
```
exec: opencode run --dir <PATH_PROJECT> -s <SESSION_ID> --command "/handoff"
```

### Fallback via opencode_task
```
opencode_task: {"prompt": "/handoff", "dir": "/path/project", "session_id": "SESI_YANG_BERJALAN"}
```

## Setelah Handoff
OpenCode akan menghasilkan ringkasan. Gunakan ringkasan itu untuk memulai session baru:
```
exec: opencode run --dir <PATH_PROJECT> "Resume: [paste ringkasan handoff]"
```

## Isi Ringkasan Handoff yang Baik
- Task Goal: tujuan akhir
- Current Status: sedang dikerjakan apa
- Completed: langkah yang sudah selesai
- Pending: langkah berikutnya
- Context Files: file yang baru diubah/dibaca

## Kesalahan Umum
- Handoff tanpa mengolah ringkasannya ke session baru: konteks hilang
- Handoff di tengah kompilasi atau test: status kerja tidak sinkron
