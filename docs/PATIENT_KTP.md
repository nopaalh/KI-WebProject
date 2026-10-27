# Schema KTP pasien

## Status dan scope

Model/schema saja. Tidak ada implementasi RC4, service enkripsi/dekripsi, repository, endpoint, atau upload foto pada perubahan ini. Ciphertext tidak otomatis terbentuk hanya karena ada field `AES`/`DES`/`RC4`.

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

Field ciphertext dan association KTP tidak diekspos melalui serialisasi JSON model. Respons API terdekripsi perlu dirancang tersendiri ketika service dibuat.

## Pekerjaan berikutnya, belum diimplementasikan

- Ambil NIK profil sebagai sumber kebenaran dan pastikan NIK KTP konsisten. Foreign key tidak membuktikan kedua NIK sama; jangan mengandalkan constraint schema untuk pengecekan plaintext terenkripsi.
- Validasi string NIK 16 digit, tanggal lahir, nilai pilihan, dan input lain sebelum enkripsi. Validasi panjang NIK tidak membuktikan identitas resmi atau kebenaran struktur wilayah/tanggal lahir.
- Validasi file foto berdasarkan isi, ukuran, dan dimensi; jangan meninggalkan upload plaintext di disk.
- Enkripsi dan autentikasi setiap field/foto dengan ketiga cipher sebelum penyimpanan; verifikasi round-trip bytes identik. Jangan simpan plaintext pada kolom bytea.
- Simpan record KTP lengkap dalam satu transaksi PostgreSQL, dengan pemeriksaan role/ownership backend.
- Bedakan nilai kosong/tidak diketahui dari data yang tidak disimpan: field schema NOT NULL, tetapi belum ada validasi isi/envelope di service.

Schema ini **tidak membuktikan enkripsi, autentikasi ciphertext, konsistensi NIK, atau izin akses sudah berjalan**. Test saat ini hanya memeriksa metadata schema GORM dan serialisasi; PostgreSQL nyata belum diuji.
