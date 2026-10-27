Portal Pasien memakai Svelte dan TypeScript untuk frontend, serta Go/Gin dan PostgreSQL untuk backend. Login menggunakan cookie HttpOnly; identitas dan peran dibaca dari `/api/users/me`. Pendaftaran publik membuat akun pasien, sedangkan akun dokter disediakan rumah sakit. Frontend saat ini sampai konfirmasi akun; dashboard dan rekam medis belum dibuat.

Vite meneruskan `/api` ke backend lokal di port 8080. Pemeriksaan tipe dan build tersedia lewat `npm run check` dan `npm run build`. `npm run check:api` menguji API lokal yang sedang berjalan dan membuat satu akun sintetis. Kontrak backend dijelaskan di [API](docs/API.md).

Ilustrasi dan logo dibuat dengan AI. Font Inter disertai lisensi OFL di `frontend/public/fonts/`.
