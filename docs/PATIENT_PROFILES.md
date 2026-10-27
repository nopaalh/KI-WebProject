# Patient Profile — create endpoint

## Status

Tersedia pada kode: **POST `/api/patients/me/profile`**. Belum ada GET, update/delete, endpoint KTP, atau frontend profil pada perubahan ini. Autentikasi memakai cookie HttpOnly `access_token` seperti endpoint lain.

NIK dan BPJS dienkripsi dengan **AES-128-CBC-HMAC-SHA256** dan **DES-CTR-HMAC-SHA256** menggunakan utilitas yang sudah ada. Setiap ciphertext didekripsi dan dibandingkan byte-per-byte sebelum insert. RC4 **belum diimplementasikan**: kolomnya NULL, bukan plaintext, ciphertext AES/DES, atau bytes placeholder.

Ini implementasi bertahap sesuai permintaan terbaru, **belum memenuhi ketentuan tiga cipher untuk seluruh data**. Berat/tinggi/umur tetap plaintext sesuai keputusan pengguna. DES dan implementasi cipher buatan tangan bersifat akademik, bukan keamanan medis produksi.

## Request

```http
POST /api/patients/me/profile
Content-Type: application/json
Cookie: access_token=<cookie dari login>
```

Contoh data sintetis:

```json
{
  "weight_kg": 65.5,
  "height_cm": 170,
  "age": 21,
  "nik": "0000000000000001",
  "bpjs_number": "0000000000001"
}
```

Validasi:

- Semua field di atas diperlukan. Berat/tinggi positif, maksimal `9999.99` sesuai kapasitas kolom numeric(6,2), maksimal dua angka desimal; ini batas penyimpanan, bukan validasi klinis.
- `age` integer 0–150; nilai 0 diterima, field hilang/null ditolak. Umur tidak bertambah otomatis.
- NIK string tepat 16 digit ASCII, nomor BPJS string tepat 13 digit ASCII. Nol di depan dipertahankan. Ini validasi format, bukan verifikasi identitas/keanggotaan resmi.
- Maksimal body 4 KiB, satu JSON object, tanpa field tambahan. `user_id`, `role`, kolom ciphertext, dan nested KTP dari klien ditolak.

Backend mengambil user ID dari JWT yang sudah divalidasi lalu membaca user/role terbaru dari database **sekali per request yang valid secara JSON**. Hanya role `patient` boleh membuat profil sendiri. Tidak ada pemilihan pemilik melalui body atau URL.

## Respons

`201 Created`:

```json
{
  "message": "Patient profile created",
  "profile": {
    "id": 12,
    "user_id": 7,
    "weight_kg": 65.5,
    "height_cm": 170,
    "age": 21
  },
  "encryption": {
    "active": ["aes", "des"],
    "pending": ["rc4"]
  }
}
```

ID hanya contoh. Respons tidak mengirim NIK/BPJS, ciphertext, password hash, atau kunci. Belum ada endpoint untuk membaca kembali profil; verifikasi dekripsi saat ini dilakukan dalam service dan tes.

| Status | Kondisi |
| --- | --- |
| `400` | JSON/field/format input tidak valid |
| `401` | Cookie/token tidak valid, ID kosong, atau user sudah dihapus |
| `403` | User bukan pasien |
| `409` | User sudah mempunyai profil; tidak ada overwrite/upsert |
| `413` | Body melebihi 4 KiB |
| `415` | Content-Type bukan application/json |
| `500` | Gangguan database/kriptografi; respons memakai pesan generik |

## Penyimpanan dan perubahan schema

Gunakan `FILE_ENCRYPTION_KEY` yang sudah ada (32 byte, dikonfigurasi sebagai 64 karakter hex). Secret/key tetap di backend. Kedua representasi disimpan bersama dalam satu INSERT dengan transaksi default GORM; kegagalan enkripsi terjadi sebelum insert. Unique `user_id` mencegah profil ganda, termasuk create bersamaan. Relasi dan constraint profil-pasien yang sudah ada tetap dipakai.

Perubahan sementara pada model: `nik_rc4` dan `bpjs_number_rc4` menjadi **nullable**. CHECK panjang yang sudah ada tetap menolak bytes kosong; NULL diperbolehkan PostgreSQL. AutoMigrate saat startup menyesuaikan nullability, tanpa file SQL migration terpisah. Sebelum memakai database bersama, koordinasikan perubahan schema dan periksa bahwa kedua kolom benar-benar nullable. Database tidak dimigrasikan dalam sesi ini.

Saat RC4 tersedia, perlu backfill data yang RC4-nya NULL dengan key yang benar dan verifikasi round-trip sebelum mempertimbangkan mengembalikan NOT NULL. Jangan mengklaim record lama sudah memakai tiga cipher secara otomatis.

Profil tetap satu-ke-satu dengan user pasien; KTP merujuk `patient_profiles.id`, bukan ciphertext NIK. Implementasi create profil ini tidak membuat KTP/BPJS dokumen secara otomatis.

## Validasi yang tersedia

- Tes route menjalankan middleware JWT, controller, dan service kriptografi sungguhan, dengan repository in-memory.
- Tes memeriksa round-trip AES/DES, RC4 nil, ownership dari JWT, role terkini, input invalid/spoofing, duplikat, error database, dan pembatasan body.
- Tes repository DryRun memeriksa SQL dan binding ciphertext, nilai NULL RC4, serta mapping duplicate/error.
- Tes schema GORM memeriksa field/constraint/nullability.

Tes tersebut bukan pengujian PostgreSQL nyata. Tidak ada smoke test/database asli yang dijalankan atau ditambahkan ke CI pada perubahan ini.
