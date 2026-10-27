# Schema profil pasien

## Status

Fondasi model/schema saja: `backend/models/patientProfile.go`, relasi dari `User`, dan pendaftaran ke `AutoMigrate`. Belum ada endpoint, service enkripsi profil, atau pembuatan profil otomatis saat register. Tidak ada database yang diubah dalam sesi implementasi ini.

Tabel dibuat oleh GORM saat backend dijalankan pada database yang dikonfigurasi. Tidak memakai migration SQL up/down terpisah untuk fitur ini.

## Kolom

| Kolom | Penyimpanan | Keterangan |
| --- | --- | --- |
| `id` | Primary key | ID profil; dapat dijadikan referensi tabel dokumen nanti |
| `user_id` | Unique, not null | Satu user maksimal satu profil; user boleh belum punya profil |
| `user_role` | Plaintext, default `patient` | Kolom teknis untuk constraint role, bukan role yang boleh dipilih klien |
| `weight_kg` | Plaintext `numeric(6,2)` | Berat dalam kg, harus positif |
| `height_cm` | Plaintext `numeric(6,2)` | Tinggi dalam cm, harus positif |
| `age` | Plaintext `smallint` | Umur dalam tahun, tidak negatif; tidak otomatis bertambah |
| `nik_aes`, `nik_des`, `nik_rc4` | `bytea` | Tiga representasi terenkripsi NIK yang sama |
| `bpjs_number_aes`, `bpjs_number_des`, `bpjs_number_rc4` | `bytea` | Tiga representasi terenkripsi nomor BPJS yang sama |

Schema awal mewajibkan seluruh field tersebut saat profil disimpan. Kolom ciphertext menolak NULL/bytes kosong, tetapi **constraint ini tidak membuktikan bytes benar-benar terenkripsi atau bahwa ketiga hasil memuat plaintext yang sama**. Validasi dan enkripsi wajib dilakukan oleh service sebelum fitur penyimpanan dibuka. Bila BPJS harus opsional, sepakati perubahan nullability dan aturan tiga representasinya sebelum menambah endpoint.

NIK dan nomor BPJS pada boundary input/dekripsi harus berupa **string**, bukan angka, agar angka nol di depan tidak hilang. Tidak ada kolom NIK/BPJS plaintext. Ukuran/format nomor dan validasi input akan ditetapkan pada layer service/API; belum tersedia sekarang.

## Relasi user dan pembatasan role

`User.PatientProfile` adalah relasi has-one. Database memakai:

- Unique key `users(id, role)`.
- Composite foreign key `patient_profiles(user_id, user_role)` → `users(id, role)`.
- CHECK `user_role = 'patient'` dan unique `patient_profiles.user_id`.

Dengan kombinasi tersebut, profil tidak dapat merujuk user dokter atau user yang tidak ada. FK memakai `ON UPDATE RESTRICT` dan `ON DELETE RESTRICT`: user yang sudah punya profil tidak dapat langsung diubah menjadi dokter atau dihapus tanpa menangani profilnya terlebih dahulu. Ini sengaja tidak memakai cascade agar perubahan role tidak mengubah kepemilikan profil secara diam-diam.

Ini aturan schema, **bukan pengganti autentikasi/otorisasi endpoint**. Endpoint nantinya harus mengambil identitas/role dari backend dan memeriksa patient ownership; jangan mempercayai `user_id`/`user_role` dari klien.

## Relasi dokumen KTP dan BPJS — belum dibuat

Belum ada tabel atau model dokumen `ktp`/`bpjs`, dan belum ada foreign key ke tabel yang tidak tersedia. Saat fitur dokumen dibangun, tabel dokumen dapat merujuk `patient_profiles.id`, bukan memakai nomor identitas terenkripsi sebagai foreign key. Kardinalitas dan aturan versi dokumen belum ditetapkan.

## Batas enkripsi dan perubahan keputusan

Atas keputusan terbaru pengguna, berat, tinggi, dan umur disimpan plaintext; hanya NIK/BPJS yang disiapkan untuk enkripsi tiga cipher. Ini **menyimpang dari kesepakatan awal melindungi seluruh data sensitif** dan berpotensi tidak memenuhi ketentuan assignment “everything stored must be protected with each cipher”. Berat/tinggi/umur dalam konteks pasien tetap data pribadi; jangan menyatakan seluruh data pasien terenkripsi atau aplikasi patuh regulasi.

AES dan utilitas DES tersedia, tetapi RC4 belum tersedia pada checkout yang diperiksa. Kolom ciphertext baru merupakan tempat penyimpanan, bukan implementasi enkripsi. Fitur penyimpanan profil perlu menunggu service enkripsi lengkap; jangan memasukkan plaintext ke kolom `bytea` atau diam-diam melewati RC4.

## Validasi

Unit test model memeriksa metadata schema GORM: nama tabel/kolom, tipe binary, check constraints, unique keys, composite foreign key, serta penyembunyian ciphertext dari serialisasi JSON. Test tersebut tidak terhubung ke PostgreSQL dan bukan bukti migrasi/constraint sudah diuji pada database nyata.
