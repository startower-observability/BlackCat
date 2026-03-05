---
name: OpenCode ULW Workflow
tags: [opencode, ultrawork, ulw, workflow]
---

# OpenCode ULW Workflow

Gunakan pola ini saat user meminta coding task lewat BlackCat.

## Kapan pakai ulw

- Gunakan `ulw` atau `ultrawork` untuk task coding kompleks tanpa planning detail panjang.
- Cocok untuk investigasi + implementasi cepat end-to-end.
- Setelah selesai, tetap lakukan verifikasi (test/build/status) sebelum melapor.

## Kapan pakai agent khusus

- `@explore` untuk pola codebase internal.
- `@librarian` untuk dokumentasi eksternal dan behavior library.
- `@oracle` untuk review arsitektur atau debugging sulit.

## Rule ringkas

1. Task sederhana: eksekusi langsung.
2. Task kompleks tanpa rencana formal: gunakan `ulw`.
3. Task besar multi-step: arahkan ke `@plan` lalu `/start-work`.
