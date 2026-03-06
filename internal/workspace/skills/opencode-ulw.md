---
name: OpenCode ULW Workflow
tags: [opencode, ultrawork, ulw, workflow]
---

# OpenCode ULW Workflow

Ultra-Work Loop (ULW) untuk task coding kompleks yang butuh iterasi mandiri. OpenCode akan loop terus sampai semua sub-task selesai.

## Kapan Menggunakan ULW
- Task multi-file yang butuh siklus buat-test-fix
- Implementasi fitur atau refactoring besar
- Bug fix yang butuh eksplorasi codebase mendalam

## Cara Menjalankan

### Langsung via exec (DIREKOMENDASIKAN)
```
exec: opencode run --dir <PATH_PROJECT> --command "/ulw-loop" "<deskripsi task detail>"
```

Contoh:
```
exec: opencode run --dir /home/gillv/projects/blackcat --command "/ulw-loop" "implementasi middleware autentikasi JWT di internal/auth, buat unit test, integrasikan ke router di main.go"
```

### Plan dulu lalu ULW (untuk task sangat kompleks)
Langkah 1 - Planning:
```
exec: opencode run --dir /home/gillv/projects/myapp --agent prometheus "analisis dan buat rencana migrasi database ke PostgreSQL"
```
Langkah 2 - Execute ULW (lanjutkan session):
```
exec: opencode run --dir /home/gillv/projects/myapp -c --command "/ulw-loop"
```

### Fallback via opencode_task (jika task >10 menit)
```
opencode_task: {"prompt": "/ulw-loop\n\nDeskripsi task", "dir": "/path/project"}
```

## PENTING
- SELALU tentukan --dir dengan path project yang benar
- Simpan path project ke core_memory setelah menemukannya
- Gunakan -c untuk melanjutkan session sebelumnya
- Gunakan -s SESSION_ID untuk resume session tertentu

## Kesalahan Umum
- Tidak menyertakan --dir: OpenCode bekerja di folder yang salah
- Tidak pakai --command "/ulw-loop": OpenCode hanya merespon biasa tanpa loop otonom
- Lupa simpan path ke memory: Sesi berikutnya lupa lokasi project
