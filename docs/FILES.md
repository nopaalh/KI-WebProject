# API File — Persistensi dan Timing AES

Panduan ini melengkapi [Dokumentasi API Backend](API.md). Implementasi saat ini hanya mendukung **AES-128-CBC-HMAC-SHA256**; daftar file milik pengguna login tersedia, tetapi belum ada RC4 atau DES, meskipun model database memiliki field path RC4/DES.

## 1. Konfigurasi dan persistensi

Backend tetap memerlukan konfigurasi database, `JWT_SECRET`, dan `SERVER_PORT` seperti pada [panduan utama](API.md#2-base-url-dan-konfigurasi). `godotenv.Load()` memuat `.env` dari working directory; jalankan backend dari `backend/` dengan file `backend/.env` yang valid.

| Environment variable | Aturan |
| --- | --- |
| `FILE_ENCRYPTION_KEY` | **Wajib**; tepat 64 karakter hexadecimal yang mewakili 32 byte master key acak. Startup berhenti jika format/panjang salah. Ini bukan password atau `JWT_SECRET`. |
| `FILE_STORAGE_DIR` | Opsional; default `storage/uploads`, relatif terhadap working directory proses backend (bila dijalankan dari `backend/`, berarti `backend/storage/uploads`). Path diubah menjadi absolut saat inisialisasi. |

Buat key **sekali**:

```sh
openssl rand -hex 32
```

Simpan output tersebut sebagai nilai `FILE_ENCRYPTION_KEY` di konfigurasi backend yang aman. Jangan membuat key baru setiap restart/deploy: file yang sudah tersimpan membutuhkan key lama yang sama. Tidak ada mekanisme rotasi/re-enkripsi otomatis. Kehilangan key berarti tidak dapat memulihkan plaintext dari ciphertext. **Backup key secara terpisah dan aman** dari database serta file terenkripsi; jangan commit key, menaruhnya di database, mengirimnya ke browser, atau memasukkannya ke environment frontend. Key hanya digunakan di backend.

Direktori storage dibuat dengan mode `0700` jika belum ada; file memakai nama hasil generate `aes-*.bin` dan dibuat melalui `os.CreateTemp` (mode `0600`). Nama upload tidak dipakai sebagai path penyimpanan. Disk menyimpan envelope terenkripsi, sedangkan database menyimpan metadata, owner, path absolut `AESPath`, dan timing. **Path disk hanya disimpan di database; tidak pernah disertakan dalam response API.**

Storage tidak didaftarkan sebagai public static hosting. Jangan mengekspos direktori ini lewat web server. Gunakan disk/volume **durable**, bukan filesystem container sementara. Backup database dan ciphertext secara konsisten, serta key secara terpisah. Saat restore/pindah deployment, pertahankan pemetaan path absolut dan direktori yang cocok dengan `FILE_STORAGE_DIR`; mengganti lokasi tidak otomatis memindahkan file atau memperbarui path database.

Upload menulis ciphertext sebelum insert metadata. Jika penulisan/penutupan file atau insert database gagal, service mencoba menutup/menghapus file yang belum berhasil dipersistenkan; insert gagal tidak dianggap upload sukses. Namun operasi disk dan database **bukan transaksi atomik**, cleanup bersifat best-effort, dan process crash di antaranya dapat meninggalkan file tanpa record. Jangan menganggap ada jaminan crash-atomic atau cleanup orphan otomatis.

### Migrasi record lama

Startup memakai `AutoMigrate` pada model `File` yang sudah ada, menambahkan kolom `aes_encrypt_time_ns` dan `aes_decrypt_time_ns` (`int64`, non-null, default `0`). Record historis mendapat nilai default `0`, bukan hasil pengukuran baru. Migrasi tidak mengenkripsi ulang atau membuat ciphertext untuk record lama.

Untuk menerapkan perubahan schema secara manual sebelum smoke test, tersedia [SQL migration up/down dan panduannya](../backend/migrations/README.md). Migration ini membutuhkan tabel `files` yang sudah ada; bukan baseline database kosong. `AutoMigrate` tetap aktif, dan aplikasi belum memakai migration runner/ledger otomatis. Jangan menjalankan migration pada database remote tanpa memastikan target dan persetujuannya.

Record lama yang hanya berisi metadata tanpa file envelope AES1 yang valid **tidak dapat didekripsi**; upload ulang file asli. Metadata masih dapat dibaca oleh pemiliknya. Nilai `algorithm` pada response adalah label implementasi saat ini, bukan bukti bahwa ciphertext record historis tersedia/valid.

## 2. Autentikasi dan kepemilikan

Keempat endpoint di bawah berada di group `/api/files` yang dilindungi `AuthMiddleware` yang sudah ada. Login melalui `POST /api/users/login` mengembalikan hanya `{"message":"Login successful"}` dan mengatur cookie JWT **`access_token` (`HttpOnly`)**. Claim ID internal bernama `userId`. Middleware membaca cookie, bukan header Bearer; lihat [alur autentikasi](API.md#5-jwt-dan-alur-autentikasi-frontend).

Query metadata, dekripsi, dan update timing dibatasi oleh ID file **dan** ID pengguna login. ID yang tidak ada maupun milik pengguna lain menghasilkan `404`, bukan membocorkan keberadaan file tersebut. Daftar file hanya memuat seluruh record milik pengguna login. Owner diambil dari `AuthMiddleware` yang sudah ada melalui cookie `access_token`, bukan input klien; query `user_id` diabaikan dan tidak dapat dipakai untuk mengakses file pengguna lain. Tidak ada parameter owner yang perlu dikirim frontend.

## 3. Endpoint

Base URL contoh: `http://localhost:8080/api` (sesuaikan `SERVER_PORT`).

### Upload — `POST /api/files`

- Body: `multipart/form-data`, field file bernama **`file`**.
- Batas byte file asli: **8 MiB = 8,388,608 byte**. Service memeriksa ukuran yang dinyatakan multipart serta membatasi byte yang benar-benar dibaca.
- Batas keseluruhan request body: **9 MiB = 9,437,184 byte**, termasuk overhead multipart. Jadi file di bawah 8 MiB tetap bisa ditolak jika total body melebihi 9 MiB.
- Browser: kirim `FormData`; jangan set `Content-Type` sendiri karena boundary harus dibuat browser.
- Service membaca plaintext, mengukur `EncryptAESFile`, mengukur `DecryptAESFile` pada envelope hasilnya, lalu memeriksa round-trip dengan `bytes.Equal` **sebelum** menyimpan ciphertext dan metadata. Upload gagal jika round-trip tidak valid.

Response sukses — `201 Created`, JSON:

```json
{
  "message": "File encrypted and saved successfully",
  "file": {
    "id": 1,
    "original_name": "laporan.pdf",
    "original_size": 4096,
    "original_type": "application/pdf",
    "algorithm": "AES-128-CBC-HMAC-SHA256",
    "aes_encrypt_time_ns": 2500000,
    "aes_decrypt_time_ns": 2700000
  }
}
```

Angka ID/ukuran/timing di atas hanya ilustrasi, bukan hasil benchmark.

### Daftar file — `GET /api/files`

Tanpa body, pagination, atau filter. Mengembalikan **seluruh record milik pengguna login**, diurutkan **ID DESC** (record terbaru diinsert terlebih dahulu), bukan berdasarkan nama atau timing.

Response sukses — `200 OK`, header `Cache-Control: no-store`, JSON:

```json
{
  "files": [
    {
      "id": 1,
      "original_name": "laporan.pdf",
      "original_size": 4096,
      "original_type": "application/pdf",
      "algorithm": "AES-128-CBC-HMAC-SHA256",
      "aes_encrypt_time_ns": 2500000,
      "aes_decrypt_time_ns": 2700000
    }
  ]
}
```

Jika tidak ada record milik pengguna, response tetap `200 OK` dengan `{"files":[]}`, bukan `null` atau `404`. Setiap item memakai **DTO metadata yang sama** dengan endpoint metadata per ID: hanya tujuh field di atas, tanpa path disk atau key. Endpoint hanya membaca metadata database, tidak membaca/memeriksa file disk dan tidak mengukur ulang timing. Record historis metadata-only tetap disertakan, meskipun tidak dapat didekripsi; timing tersimpan (termasuk default historis `0`) dikembalikan apa adanya.

Error `401` memakai pesan `AuthMiddleware` yang sudah ada seperti pada tabel error di bawah. Kegagalan pemrosesan/database menghasilkan `500 Internal Server Error` dengan `{"message":"Failed to process file"}`.

### Metadata — `GET /api/files/:id`

Tanpa body. `:id` harus berupa bilangan bulat positif. Response sukses — `200 OK`, JSON dengan satu properti `file` yang memiliki field sama seperti upload, tanpa properti `message`:

```json
{
  "file": {
    "id": 1,
    "original_name": "laporan.pdf",
    "original_size": 4096,
    "original_type": "application/pdf",
    "algorithm": "AES-128-CBC-HMAC-SHA256",
    "aes_encrypt_time_ns": 2500000,
    "aes_decrypt_time_ns": 2700000
  }
}
```

| Field | Makna |
| --- | --- |
| `id` | ID record; diperoleh dari response upload atau daftar file milik pengguna login. |
| `original_name` | Nama asli yang diambil basename-nya, bukan path upload pengguna. |
| `original_size` | Ukuran plaintext dalam byte, bukan ukuran ciphertext. |
| `original_type` | `Content-Type` dari bagian multipart upload; metadata klien, bukan hasil verifikasi tipe isi. |
| `algorithm` | Label tetap `AES-128-CBC-HMAC-SHA256` dari implementasi saat ini. |
| `aes_encrypt_time_ns` | Durasi enkripsi saat upload, tersimpan di database. |
| `aes_decrypt_time_ns` | Awalnya durasi dekripsi verifikasi saat upload; setelah download dekripsi sukses, nilai terakhir yang dipersistenkan. |

Response menggunakan DTO khusus; tidak menyertakan `AESPath`, path RC4/DES, master key, atau byte ciphertext. GET hanya membaca metadata; tidak mengukur ulang maupun memeriksa keberadaan/integritas file disk.

### Dekripsi dan download — `POST /api/files/:id/decrypt`

Tanpa body. Service mengambil record milik pengguna, membaca envelope dari path storage yang diizinkan, mendekripsi dan memverifikasi integritas serta ukuran plaintext, lalu memperbarui **`aes_decrypt_time_ns` di database sebelum mengirim response sukses**. Jika update database gagal, request gagal, bukan mengirim download sukses tanpa timing tersimpan.

Response sukses — `200 OK` dengan **byte asli file**, bukan JSON, base64, atau envelope terenkripsi:

```http
Content-Type: application/octet-stream
Content-Disposition: attachment; filename=laporan.pdf
X-Decryption-Time-Ns: 2800000
X-Content-Type-Options: nosniff
Cache-Control: no-store
```

`Content-Disposition` diformat oleh backend menggunakan nama asli tersanitasi; quoting/encoding dapat berbeda untuk nama yang mengandung spasi atau karakter non-ASCII. `X-Decryption-Time-Ns` adalah integer desimal dalam nanodetik untuk dekripsi pada request ini, sama dengan nilai yang dipersistenkan oleh request tersebut. Request dekripsi berikutnya **menimpa** nilai lama; tidak ada riwayat/akumulasi/rata-rata, dan request bersamaan dapat menimpa nilai satu sama lain. `aes_encrypt_time_ns` tidak berubah. Ambil metadata lagi untuk membaca nilai dekripsi terakhir yang tersimpan.

Upload, daftar file, dan metadata juga mengatur `Cache-Control: no-store`. Jangan mengandalkan seluruh response error middleware memiliki header tersebut.

### Error

Error endpoint file berbentuk JSON `{"message":"..."}`, termasuk pada route dekripsi yang suksesnya binary. Periksa status HTTP sebelum membaca body.

| Status | Kondisi / `message` |
| --- | --- |
| `400 Bad Request` | Field file hilang: `file is required`; multipart tidak valid: `Invalid multipart request`; ID nol/tidak valid: `Invalid file ID`. |
| `401 Unauthorized` | Cookie hilang: `Missing authentication cookie`; JWT invalid/kedaluwarsa: `Invalid or expired token`; claim ID tidak valid: `Unauthorized: user_id missing`; controller juga dapat mengembalikan `Unauthorized`. |
| `404 Not Found` | Record tidak ada atau bukan milik pengguna: `file not found`. |
| `413 Request Entity Too Large` | File melebihi 8 MiB atau body melebihi batas 9 MiB: `file size exceeds 8 MiB limit`. |
| `500 Internal Server Error` | Kegagalan I/O/database, ciphertext hilang/rusak, key berbeda, record metadata-only lama, atau verifikasi gagal: `Failed to process file`. Detail internal tidak dikirim ke klien. |

## 4. Arti dan batasan timing

Service memakai `time.Now()` / `time.Since(...).Nanoseconds()` tepat di sekitar pemanggilan fungsi envelope:

| Nilai | Operasi yang diukur |
| --- | --- |
| `aes_encrypt_time_ns` saat upload | Satu pemanggilan `utils.EncryptAESFile(plaintext, key)`. |
| `aes_decrypt_time_ns` saat upload | Satu pemanggilan `utils.DecryptAESFile(encrypted, key)` untuk verifikasi round-trip sebelum persistensi. |
| Header `X-Decryption-Time-Ns` dan update `aes_decrypt_time_ns` saat dekripsi | Satu pemanggilan `utils.DecryptAESFile(encrypted, key)` setelah ciphertext selesai dibaca dari disk. |

**Termasuk** pekerjaan di dalam fungsi envelope: derivasi key, alokasi/copy buffer, ekspansi key AES, CBC, padding PKCS7/pemeriksaannya, dan pembuatan/verifikasi HMAC; enkripsi juga mencakup pembuatan IV acak. Ini bukan timing hanya putaran AES atau satu blok AES.

**Tidak termasuk** transfer upload/download, parsing multipart, pembacaan plaintext upload atau ciphertext disk, penulisan file, query/insert/update database, serialisasi response, maupun perbandingan `bytes.Equal` saat verifikasi upload. Pemeriksaan panjang plaintext setelah dekripsi download juga di luar timer. Latensi HTTP yang dirasakan pengguna akan berbeda dan umumnya mencakup pekerjaan tambahan tersebut.

Konversi tampilan: **`milliseconds = nanoseconds / 1e6`**. Misalnya `2500000 ns` berarti `2.5 ms`. Setiap nilai adalah **satu sampel wall-clock**, bukan benchmark ketat atau rata-rata. Ukuran/isi file, CPU, scheduler, garbage collection, dan beban proses dapat memengaruhi hasil. Jangan menyimpulkan performa algoritma dari satu upload/download atau membandingkannya dengan RC4/DES yang belum diimplementasikan. Nilai historis `0` adalah default migrasi, bukan bukti operasi berlangsung nol waktu.

## 5. Format envelope dan batas keamanan

Format disk:

```text
"AES1" (4 byte) || IV acak (16 byte) || ciphertext AES-128-CBC + PKCS7 || HMAC-SHA256 (32 byte)
```

Ciphertext adalah hasil enkripsi plaintext yang sudah diberi padding PKCS7, panjangnya kelipatan 16 byte, termasuk satu blok padding penuh jika plaintext sudah selaras. HMAC mengautentikasi `magic || IV || ciphertext` dan diverifikasi **sebelum** dekripsi AES/pemeriksaan padding. Master key 32 byte diturunkan menjadi:

- Key AES 16 byte: 16 byte pertama `HMAC-SHA256(masterKey, "AES1/encryption")`.
- Key MAC 32 byte: `HMAC-SHA256(masterKey, "AES1/authentication")`.

**AES buatan tangan ini untuk pembelajaran dan tidak constant-time; bukan jaminan keamanan produksi.** Keberadaan HMAC dan verifikasi integritas tidak membuat implementasi AES edukasional ini siap melindungi data produksi. Belum ada RC4 atau DES pada alur file ini.

## 6. Contoh curl dengan cookie jar

Asumsi pengguna sudah register dan backend memakai port `8080`. Sesuaikan path file lokal; contoh ID `1` harus diganti dengan `file.id` dari response upload.

```sh
# Login: simpan cookie HttpOnly ke cookie jar lokal.
curl -i -c cookies.txt -X POST http://localhost:8080/api/users/login \
  -H 'Content-Type: application/json' \
  --data '{"username":"nopal","password":"contoh-password-kuat"}'

# Upload: curl membuat Content-Type multipart beserta boundary.
curl -i -b cookies.txt http://localhost:8080/api/files \
  -F 'file=@./laporan.pdf'

# Daftar seluruh file milik pengguna cookie ini, urutan ID DESC, tanpa pagination/filter.
curl -i -b cookies.txt http://localhost:8080/api/files

# Baca metadata dan timing yang dipersistenkan.
curl -i -b cookies.txt http://localhost:8080/api/files/1

# Dekripsi: simpan byte asli dan header (termasuk timing) secara terpisah.
curl --fail-with-body -b cookies.txt -X POST \
  http://localhost:8080/api/files/1/decrypt \
  -D decrypt-headers.txt -o laporan-decrypted.pdf

# Opsional: bandingkan hasil download dengan file asli setelah HTTP sukses.
cmp ./laporan.pdf ./laporan-decrypted.pdf

# Baca lagi untuk melihat aes_decrypt_time_ns terbaru.
curl -i -b cookies.txt http://localhost:8080/api/files/1
```

Periksa keberhasilan curl/status sebelum menjalankan `cmp`: `--fail-with-body` memberi exit code gagal pada HTTP error tetapi tetap dapat menyimpan body **JSON error** ke file output. Lindungi dan hapus `cookies.txt` setelah pengujian karena berisi kredensial autentikasi; jangan commit cookie jar atau hasil download sensitif.

## 7. Contoh frontend: FormData dan binary download

Contoh mengasumsikan login sudah berhasil. Base URL ini untuk akses langsung dari `http://localhost:5173`; sesuaikan port atau gunakan `/api` jika memakai proxy/same-origin.

```js
const API_BASE_URL = "http://localhost:8080/api";

async function throwFileError(response) {
  const data = await response.json().catch(() => null);
  const error = new Error(data?.message ?? `Request gagal (HTTP ${response.status})`);
  error.status = response.status;
  throw error;
}

async function uploadFile(file) {
  const form = new FormData();
  form.append("file", file);
  const response = await fetch(`${API_BASE_URL}/files`, {
    method: "POST",
    credentials: "include",
    body: form,
    // Jangan set Content-Type; browser menambahkan boundary multipart.
  });
  if (!response.ok) await throwFileError(response);
  return (await response.json()).file;
}

async function listFiles() {
  const response = await fetch(`${API_BASE_URL}/files`, {
    method: "GET",
    credentials: "include",
  });
  if (!response.ok) await throwFileError(response);
  return (await response.json()).files; // Array metadata ID DESC; [] jika kosong.
}

async function getFileMetadata(id) {
  const response = await fetch(`${API_BASE_URL}/files/${id}`, {
    credentials: "include",
  });
  if (!response.ok) await throwFileError(response);
  return (await response.json()).file;
}

async function downloadDecryptedFile(file) {
  const response = await fetch(`${API_BASE_URL}/files/${file.id}/decrypt`, {
    method: "POST",
    credentials: "include",
  });
  // Error berupa JSON; sukses berupa byte file. Jangan panggil json() saat sukses.
  if (!response.ok) await throwFileError(response);
  const timing = response.headers.get("X-Decryption-Time-Ns");
  const decryptTimeMs = timing === null ? null : Number(timing) / 1e6;
  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = file.original_name;
  document.body.append(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
  return decryptTimeMs;
}
```

Untuk `fetch`, header `attachment` tidak otomatis memulai download; contoh membuat object URL dari `response.blob()`. Router mengekspos `Content-Disposition` dan `X-Decryption-Time-Ns` melalui CORS, sehingga frontend juga dapat membaca header nama file/timing. Contoh memakai `original_name` dari metadata agar tidak perlu parsing `Content-Disposition`. Tangani `401` dengan meminta login ulang dan kegagalan jaringan/CORS secara terpisah dari HTTP error. Login lintas origin juga wajib memakai `credentials: "include"`; CORS saat ini hanya mengizinkan `http://localhost:5173` dengan credentials aktif.
