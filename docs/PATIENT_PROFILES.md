# Patient Profile — penyimpanan dan implementasi

## Kontrak API

Kontrak request/response, validasi field, autentikasi/ownership, status error, serta contoh data sintetis untuk **POST dan GET `/api/patients/me/profile`** dipusatkan di **[API.md — bagian 9: Profil pasien](API.md#9-profil-pasien)**.

Dokumen ini hanya membahas penyimpanan dan implementasi agar kontrak tidak diduplikasi. Belum ada update/delete profil, create/upload KTP, atau frontend profil pada perubahan ini. Endpoint baca data KTP tersedia; endpoint foto KTP belum tersedia; kontraknya di [API.md — bagian 10](API.md#10-ktp-pasien).

## Enkripsi dan batas perlindungan

Record profil baru menyimpan NIK dan BPJS sebagai tiga representasi terenkripsi: **AES-128-CBC-HMAC-SHA256**, **DES-CTR-HMAC-SHA256**, dan **RC4-drop1536-HMAC-SHA256**. RC4 memakai envelope `RC41`, nonce acak 16 byte, serta kunci enkripsi turunan per nonce. Setiap ciphertext didekripsi dan dibandingkan byte-per-byte sebelum insert; jika salah satu gagal, profil tidak disimpan.

Implementasi ini **belum memenuhi ketentuan tiga cipher untuk seluruh data**: berat/tinggi/umur tetap plaintext sesuai keputusan pengguna. DES, RC4, dan implementasi cipher buatan tangan bersifat akademik, bukan keamanan medis produksi.

## Penyimpanan dan perubahan schema

Gunakan `FILE_ENCRYPTION_KEY` yang sudah ada (32 byte, dikonfigurasi sebagai 64 karakter hex). Secret/key tetap di backend. Ketiga representasi disimpan bersama dalam satu INSERT dengan transaksi default GORM; kegagalan enkripsi terjadi sebelum insert. Unique `user_id` mencegah profil ganda, termasuk create bersamaan. Relasi dan constraint profil-pasien yang sudah ada tetap dipakai.

Kolom `nik_rc4` dan `bpjs_number_rc4` tetap **nullable untuk kompatibilitas record lama** dari tahap AES/DES. Record baru dari endpoint create selalu mengisi kedua kolom RC4; tidak memakai NULL atau placeholder. CHECK panjang yang sudah ada menolak bytes kosong; NULL pada record historis masih diperbolehkan PostgreSQL.

Record lama yang RC4-nya NULL belum otomatis terlindungi RC4. Perlu backfill terpisah dengan key yang benar dan verifikasi round-trip sebelum mempertimbangkan mengembalikan NOT NULL. Backfill tidak disertakan atau dijalankan pada perubahan ini. Aturan pembacaan record lama dijelaskan dalam kontrak GET di API.md.

Profil tetap satu-ke-satu dengan user pasien; KTP merujuk `patient_profiles.id`, bukan ciphertext NIK. Endpoint profil tidak membuat dokumen KTP/BPJS secara otomatis. Model/schema memakai AutoMigrate yang sudah ada; dokumentasi ini tidak menjalankan migrasi database.

## Validasi yang tersedia

- Tes route menjalankan middleware JWT, controller, dan service kriptografi sungguhan, dengan repository in-memory.
- Tes create memeriksa round-trip AES/DES/RC4, penolakan RC4 yang dimodifikasi, ownership, role terkini, input invalid/spoofing, duplikat, error database, dan pembatasan body.
- Tes GET memeriksa create→read, ownership/query spoofing, role terbaru, profil tidak ditemukan, ciphertext rusak/tidak cocok, kunci salah, kompatibilitas record lama, dan tidak adanya respons sebagian.
- Tes repository DryRun memeriksa SQL/binding ketiga cipher, query read yang dibatasi user ID, dan mapping duplicate/error.
- Tes schema GORM memeriksa field/constraint/nullability.

Tes tersebut bukan pengujian PostgreSQL nyata. Tidak ada smoke test/database asli yang dijalankan atau ditambahkan ke CI pada perubahan ini.
