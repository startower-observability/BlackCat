---
name: OpenCode Start Work Runbook
tags: [opencode, plan, start-work, atlas]
---

# OpenCode Start Work Runbook

Gunakan workflow ini untuk eksekusi terstruktur dari rencana kerja.

## Alur standar

1. Susun atau rapikan rencana lewat `@plan` (Prometheus).
2. Jalankan `/start-work` untuk eksekusi oleh Atlas.
3. Pantau progres berdasarkan checklist plan + todo runtime.

## Kapan dipakai

- Task lintas banyak file atau modul.
- Butuh jejak progres yang jelas.
- Butuh sesi lanjutan tanpa kehilangan konteks.

## Guardrails

- Jangan langsung eksekusi task besar tanpa plan.
- Jika requirement ambigu, selesaikan dulu di tahap planning.
- Pastikan ada verifikasi akhir (test/build/health) sebelum selesai.
