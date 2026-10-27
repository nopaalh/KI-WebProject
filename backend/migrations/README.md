# Migration PostgreSQL

Folder ini menyimpan perubahan schema SQL bernomor yang bisa di-review melalui Git. Saat ini migration dijalankan **manual** dengan `psql`; belum ada migration runner atau tabel pencatat versi di database. Urutan nama file bukan bukti bahwa sebuah migration sudah diterapkan.

## Cakupan dan prasyarat

`000001_add_aes_processing_times` adalah migration **incremental untuk database yang sudah memiliki tabel `files`**, bukan baseline schema untuk database kosong. Tabel `users`/`files` sebelumnya dikelola oleh GORM `AutoMigrate` pada `backend/main.go`.

Migration up menambahkan:

| Kolom | Tipe PostgreSQL | Nullable | Default | Satuan |
| --- | --- | --- | --- | --- |
| `aes_encrypt_time_ns` | `BIGINT` | Tidak | `0` | Nanodetik |
| `aes_decrypt_time_ns` | `BIGINT` | Tidak | `0` | Nanodetik |

Definisi ini mengikuti field `int64` dan tag GORM pada `backend/models/file.go`. Record lama mendapat nilai `0`; migration tidak mengukur ulang waktu, mengenkripsi file lama, atau mengubah ciphertext/path yang sudah ada.

`ADD COLUMN IF NOT EXISTS` mengakomodasi kolom yang mungkin sudah dibuat `AutoMigrate`. Namun opsi ini **tidak memperbaiki** tipe/default/nullability kolom yang sudah ada. Periksa hasil verifikasi, jangan menganggap notice `already exists` sebagai validasi schema.

## Hubungan dengan AutoMigrate

**AutoMigrate pada startup tetap aktif dan tidak diubah oleh penambahan file SQL ini.** File migration tidak dijalankan otomatis oleh aplikasi. Ini menyediakan langkah schema yang bisa diperiksa dan diterapkan sebelum smoke test; belum merupakan perpindahan penuh ke sistem migration berversi.

Untuk database kosong, diperlukan baseline schema terlebih dahulu; jangan menjalankan migration ini dengan asumsi ia membuat semua tabel. Untuk deployment yang sepenuhnya mengandalkan migration, baseline dan penggantian alur `AutoMigrate` perlu direncanakan terpisah.

## Sebelum menjalankan

1. Pastikan database target memang database development/test yang diizinkan. Jangan menjalankan pada database remote/production tanpa persetujuan dan backup.
2. Periksa database, schema, dan keberadaan tabel `files`. SQL memakai nama tabel tanpa prefix schema, mengikuti `search_path` koneksi seperti GORM; pastikan mengarah ke tabel yang benar.
3. Gunakan koneksi yang sama dengan `DATABASE_URL` backend. `psql` tidak membaca file `.env` otomatis. Jangan memakai `source .env` pada file yang belum diperiksa atau menampilkan URL yang mengandung password di log/chat.
4. Gunakan prompt password atau konfigurasi credential PostgreSQL lokal yang aman. Jangan memasukkan password ke command yang akan tersimpan di shell history.
5. Jalankan dari direktori `backend/` agar path file di bawah sesuai.

Contoh di bawah memakai placeholder `YOUR_HOST`, `YOUR_DATABASE`, dan `YOUR_USER`; ganti dengan konfigurasi database yang benar. `sslmode=require` ditujukan untuk koneksi remote yang mendukung TLS; ikuti ketentuan provider, termasuk verifikasi sertifikat bila diperlukan. Untuk PostgreSQL lokal, sesuaikan opsi TLS dengan setup lokal.

## Terapkan migration up

```sh
psql "host=YOUR_HOST port=5432 dbname=YOUR_DATABASE user=YOUR_USER sslmode=require" \
  -X -W --set=ON_ERROR_STOP=1 \
  --file=migrations/000001_add_aes_processing_times.up.sql
```

- `-X` menghindari efek konfigurasi `psqlrc` lokal.
- `-W` meminta password secara interaktif; jangan menjalankan contoh ini melalui proses non-interaktif yang tidak bisa menjawab prompt.
- `ON_ERROR_STOP` menghentikan proses jika SQL gagal.
- `BEGIN`/`COMMIT` membuat kedua perubahan kolom berada dalam satu transaksi.
- `lock_timeout = '5s'` membatasi penantian lock tabel. Jika timeout, investigasi transaksi yang menahan lock dan coba lagi setelah aman; jangan menganggap migration sudah selesai.

`ALTER TABLE` tetap membutuhkan lock dan dapat memengaruhi request lain. Jadwalkan perubahan sesuai kondisi database, terutama pada lingkungan bersama.

## Verifikasi schema

Melalui koneksi `psql` yang sama, jalankan:

```sql
SELECT current_database(), current_schema();

SELECT to_regclass('files') AS target_table;

SELECT column_name, data_type, is_nullable, column_default
FROM information_schema.columns
WHERE table_schema = current_schema()
  AND table_name = 'files'
  AND column_name IN ('aes_encrypt_time_ns', 'aes_decrypt_time_ns')
ORDER BY column_name;
```

Harus ada dua kolom bertipe `bigint`, `is_nullable = 'NO'`, dan default numerik nol. PostgreSQL dapat menampilkan default dengan cast, misalnya `0` atau `0::bigint`. Jika memakai `search_path` khusus, sesuaikan filter schema dengan lokasi tabel hasil `to_regclass`.

Catat migration yang diterapkan, database target, waktu, dan commit Git terkait pada catatan deployment. Folder ini belum menyimpan ledger migration otomatis.

## Rollback

**Destruktif:** rollback menghapus kedua kolom beserta seluruh nilai timing di dalamnya, termasuk bila kolom itu awalnya dibuat oleh `AutoMigrate` sebelum SQL up dijalankan. Ia tidak dapat menentukan siapa yang pertama kali menambahkan kolom tersebut. Backup nilai yang diperlukan terlebih dahulu.

Hentikan atau rollback aplikasi yang masih membaca/menulis field timing sebelum menghapus kolom. Jangan menyalakan kembali versi aplikasi sekarang setelah rollback schema saja: `AutoMigrate` akan membuat kolom tersebut lagi, sementara data timing lama tetap hilang.

```sh
psql "host=YOUR_HOST port=5432 dbname=YOUR_DATABASE user=YOUR_USER sslmode=require" \
  -X -W --set=ON_ERROR_STOP=1 \
  --file=migrations/000001_add_aes_processing_times.down.sql
```

Rollback ini tidak menghapus record file, pengguna, atau ciphertext di disk.

## Setelah schema siap

Untuk validasi lokal terisolasi, jalankan [script smoke test Docker](../../docs/SMOKE_TEST.md). Script memakai PostgreSQL sementara, menguji migration up/down serta alur API, dan tidak membaca `.env` aplikasi atau menyentuh database remote.

Untuk menguji database aplikasi yang sudah diizinkan, ikuti [panduan alur file](../../docs/FILES.md): pastikan key backend sudah dikonfigurasi, lalu register/login, upload, periksa metadata dan ciphertext, dekripsi dan bandingkan byte, kemudian restart dengan key yang sama dan dekripsi ulang. Verifikasi schema saja belum membuktikan alur tersebut berjalan.
