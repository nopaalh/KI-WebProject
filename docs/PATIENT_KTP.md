# Schema KTP pasien

## Status dan scope

Model/schema serta endpoint baca data dan foto tersedia: `GET /api/patients/me/ktp` dan `GET /api/patients/me/ktp/photo`. Service memakai utilitas AES/DES/RC4 yang sudah ada untuk dekripsi dan memeriksa kesamaan hasil; repository membatasi user pemilik melalui relasi profil.

**POST `/api/patients/me/ktp` tersedia** untuk data JSON dan foto PNG/JPEG melalui multipart. Kedua GET membaca hasil yang sudah tersimpan. Fixture dan upload pada tes memakai data sintetis, bukan identitas asli. Kontrak request/response, error, serta status kelima endpoint pasien dipusatkan di [API.md — bagian 10](API.md#10-ktp-pasien).

`PatientKTP` didaftarkan ke `AutoMigrate` yang sudah ada. Tabel akan dibuat saat backend dijalankan terhadap database yang dikonfigurasi; migration tidak dijalankan dalam sesi ini. Tidak ada SQL migration terpisah.

## Relasi

```text
users.id
  ← patient_profiles.user_id
      patient_profiles.id
        ← patient_ktps.patient_profile_id (UNIQUE, NOT NULL)
```

Satu profil maksimal satu KTP; profil boleh belum punya KTP. Relasi profil ke user pasien mengikuti constraint model profil yang sudah ada. Foreign key KTP memakai ID profil, bukan NIK terenkripsi: IV/nonce acak membuat ciphertext NIK tidak cocok dijadikan kunci relasi.

FK memakai ON UPDATE/DELETE RESTRICT agar penghapusan/perubahan profil tidak diam-diam menghapus dokumen identitas.

## Kolom terenkripsi

`KTPEncryptedValue` adalah embedded struct GORM, **bukan tabel baru atau satu kolom JSON**. Contoh field `FullName` menjadi `full_name_aes`, `full_name_des`, dan `full_name_rc4` di PostgreSQL.

Setiap field berikut memiliki tiga kolom `bytea NOT NULL` dengan suffix `_aes`, `_des`, `_rc4`:

| Prefix | Isi sebelum enkripsi |
| --- | --- |
| `nik` | NIK sebagai string 16 digit |
| `full_name` | Nama lengkap |
| `birth_place` | Tempat lahir |
| `birth_date` | Tanggal lahir |
| `gender` | Jenis kelamin |
| `blood_type` | Golongan darah, termasuk nilai tidak diketahui bila diperlukan |
| `address` | Alamat jalan/detail alamat |
| `rt` | RT sebagai string, menjaga nol di depan |
| `rw` | RW sebagai string, menjaga nol di depan |
| `village` | Kelurahan/desa |
| `district` | Kecamatan |
| `religion` | Agama |
| `marital_status` | Status perkawinan |
| `occupation` | Pekerjaan |
| `citizenship` | WNI/WNA |
| `photo` | Bytes asli foto KTP |

Total: **45 kolom ciphertext teks + 3 kolom ciphertext foto**, di samping `id` dan `patient_profile_id`. Foto dienkripsi lalu ciphertext-nya disimpan langsung di PostgreSQL, bukan filesystem, path, atau Base64.

Field ciphertext dan association KTP tidak diekspos melalui serialisasi JSON model. Respons API terdekripsi memakai DTO tersendiri; kontraknya ada di API.md.

## Alur create yang tersedia

- NIK diambil dari profil milik pasien login; NIK/ID pemilik dari klien tidak diterima. Profil dibaca melalui service yang mengecek role terbaru dan integritas representasi identitas.
- Data teks, tanggal lahir, pilihan field, dan foto divalidasi sebelum enkripsi. Foto dibatasi 8 MiB/25 juta pixel lalu di-decode penuh untuk memastikan format dapat dibaca; ini tidak membuktikan identitas resmi atau keaslian KTP.
- Multipart dibaca terbatas di memori, tanpa temporary upload plaintext di disk. Original filename tidak disimpan.
- Seluruh 15 field dan foto dienkripsi dengan AES/DES/RC4, diverifikasi round-trip, dan disimpan dalam satu INSERT dengan transaksi default GORM. Foto tetap bytea di PostgreSQL.
- Unique patient_profile_id menolak duplikat; tidak ada overwrite/upsert. Update/delete belum tersedia.
- Kontrak validasi, batas ukuran, dan response berada di API.md bagian 10.7.

Tes mencakup metadata schema/serialisasi, query owner-scoped dengan DryRun, endpoint read menggunakan cipher sungguhan dan repository in-memory, penolakan akses tidak sah, ciphertext rusak/tidak cocok, serta NIK yang berbeda. Tes tambahan mencakup POST multipart → GET teks/foto dengan cipher sungguhan, duplicate/error mapping, dan INSERT DryRun yang mengikat seluruh 48 kolom ciphertext. PostgreSQL nyata belum diuji.
