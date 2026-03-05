---
name: OpenCode Handoff Quality
tags: [opencode, handoff, continuity, context]
---

# OpenCode Handoff Quality

Gunakan `/handoff` saat perlu pindah sesi tanpa kehilangan konteks kerja.

## Kapan wajib handoff

- Context sesi sudah panjang.
- Task belum selesai dan akan dilanjutkan di sesi baru.
- Perlu transfer status kerja ke agent atau sesi lain.

## Isi minimal handoff

- User request asli (verbatim).
- Goal dan status saat ini.
- Item yang sudah selesai vs pending.
- File penting yang disentuh.
- Constraint eksplisit (jangan ditafsirkan ulang).

## Rule kualitas

- Fokus pada informasi lanjutan yang actionable.
- Hindari narasi panjang yang tidak membantu eksekusi sesi berikutnya.
- Jangan bocorkan secret atau credential.
