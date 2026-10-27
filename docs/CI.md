# CI dan auto-merge PR

## Tujuan dan status

Push branch → buka PR ke `main` → CI berjalan → satu anggota lain memberikan approval → GitHub otomatis squash merge setelah semua syarat branch protection terpenuhi.

Workflow disiapkan di repo, **belum aktif di GitHub sebelum di-commit/push dan setting repo dikonfigurasi**. Ini CI dan otomatisasi merge, bukan deployment/CD. Tidak ada server produksi, credential database aplikasi, atau encryption key aplikasi yang diperlukan.

## Workflow

### `.github/workflows/ci.yml`

Berjalan pada setiap PR ke `main`, push ke `main`, dan manual dispatch. Tidak memakai path filter supaya required checks tetap muncul pada PR dokumentasi.

Dua job menjalankan pemeriksaan backend dan frontend:

| Required check | Isi |
| --- | --- |
| `Backend checks` | Go formatting, `go mod verify`, `go vet`, `go test -race` |
| `Frontend checks` | Node.js 24, `npm ci`, Svelte/TypeScript checks, Vite production build |

Go mengikuti `backend/go.mod`. Frontend memakai lockfile npm dan dibangun tanpa koneksi API. Status `Frontend checks` hanya memblokir merge jika diwajibkan oleh branch protection; auto-merge yang ada memeriksa `Backend checks`. CI tidak menjalankan smoke test, Python smoke harness, Docker, atau PostgreSQL nyata. Smoke test tetap dapat dijalankan manual sesuai [SMOKE_TEST.md](SMOKE_TEST.md). CI hijau tidak membuktikan integrasi PostgreSQL atau seluruh rancangan rumah sakit sudah berjalan.

### `.github/workflows/auto-merge.yml`

Meminta GitHub mengaktifkan auto-merge dengan metode **squash**, bukan memanggil merge paksa atau melewati review/checks.

Pengaman:

- Harus ada repository variable `AUTO_MERGE_ENABLED=true`.
- Hanya PR terbuka, bukan draft, dari branch di repo yang sama, dengan tujuan `main`.
- Memeriksa bahwa classic branch protection mewajibkan minimal satu approval dan required check `Backend checks`. Jika belum benar, workflow gagal tanpa mengaktifkan auto-merge.
- Menggunakan `GITHUB_TOKEN` bawaan dengan `contents: write` dan `pull-requests: write`; tidak perlu PAT.
- `pull_request_target` dipakai agar konfigurasi berasal dari base branch. **Jangan menambahkan checkout PR, menjalankan script PR, atau memasukkan input PR ke shell dalam workflow berizin tulis ini.**
- Nama required checks harus unik; jangan membuat workflow lain dengan nama job yang sama.

PR fork tidak di-auto-merge. Ini sengaja dibatasi untuk alur branch bersama tim. PR tetap perlu dibuat dan di-review; workflow tidak membuat PR atau memberikan approval sendiri.

## Setup satu kali oleh admin repo bersama

Target: `nopaalh/KI-WebProject`, bukan fork pribadi. Penambahan file workflow sendiri tidak mengubah setting GitHub.

1. Masukkan workflow ke repo lewat PR awal yang di-review manual. CI tidak bergantung pada script smoke test.
2. Jalankan CI pertama agar nama check terdaftar di GitHub. Lihat **Actions** atau checks pada PR.
3. Di **Settings → General → Pull Requests**, aktifkan **Allow squash merging** dan **Allow auto-merge**.
4. Di **Settings → Branches**, buat **classic branch protection rule** untuk `main`:
   - **Require a pull request before merging**.
   - **Require approvals**, minimal **1**. Penulis PR tidak dapat menyetujui PR sendiri.
   - **Dismiss stale pull request approvals when new commits are pushed**, supaya perubahan baru di-review lagi.
   - **Require status checks to pass before merging**; pilih persis `Backend checks` dari GitHub Actions. Jika `PostgreSQL smoke test` sebelumnya sudah diwajibkan, hapus dari daftar required checks agar PR tidak menunggu check yang tidak lagi dijalankan.
   - **Require branches to be up to date before merging** agar perubahan terbaru `main` ikut diuji. Jika branch ketinggalan, author tetap perlu update/rebase branch; workflow ini tidak mengubah branch otomatis.
   - **Do not allow bypassing the above settings** (atau opsi ekuivalen) agar admin/bot tidak melewati pemeriksaan. Jangan beri actor workflow bypass.
5. Di **Settings → Secrets and variables → Actions → Variables**, tambahkan **repository variable**, bukan secret: `AUTO_MERGE_ENABLED` = `true`.
6. Pastikan kebijakan GitHub Actions repo/organisasi mengizinkan workflow memakai `contents: write` dan `pull-requests: write`. Tidak perlu mengaktifkan izin membuat/menyetujui PR oleh Actions; workflow ini tidak melakukan approval.

Workflow ini memeriksa **classic branch protection**, bukan konfigurasi yang hanya memakai Rulesets. Ketersediaan protection/auto-merge bisa bergantung pada visibilitas repo, plan, dan kebijakan organisasi. Jika setting tidak tersedia, jangan melemahkan checks; CI tetap bisa digunakan dengan merge manual.

## Penggunaan harian

1. Kerja di branch fitur dan push ke repo bersama.
2. Buka PR ke `main`; gunakan judul bermakna karena akan menjadi judul squash commit.
3. CI dan workflow pengaktif auto-merge berjalan otomatis.
4. Satu anggota lain review dan approve. Kalau ada test gagal atau perubahan diminta, perbaiki dulu. Push baru menjalankan ulang CI dan, dengan setting di atas, membatalkan approval lama.
5. Setelah syarat lengkap, GitHub merge tanpa perlu menekan tombol merge lagi.

Untuk PR yang sudah ada sebelum toggle aktif: **Actions → Enable PR auto-merge → Run workflow**, pilih branch `main`, masukkan nomor PR. Workflow pengaktif harus sudah berada di default branch agar manual dispatch tersedia. Jangan memberikan approval sebelum checks/setting siap saat setup awal.

Untuk mematikan otomatisasinya: ubah `AUTO_MERGE_ENABLED` ke `false`. **Ini hanya mencegah aktivasi baru**; batalkan juga auto-merge yang sudah aktif pada masing-masing PR jika ingin menghentikannya.

## Verifikasi dan batasan

- Pengujian lokal Go tidak membuktikan workflow GitHub atau auto-merge berhasil; validasi akhir harus lewat PR uji setelah setup repo.
- Pastikan perubahan yang menyebabkan tes gagal benar-benar memblokir merge; PR belum approved juga harus tetap tertahan.
- Jangan menyimpan secret aplikasi di workflow atau mengunggah `.env`, key, cookie, maupun plaintext pasien sebagai artifact.
- GitHub Actions dapat mengonsumsi kuota/menit runner sesuai plan repo.
- Workflow tidak push langsung ke `main`, tidak deploy, tidak menghilangkan kemungkinan konflik merge, dan tidak menggantikan review keamanan.
