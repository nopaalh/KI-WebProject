# Dokumentasi API Backend — Panduan FrontEnd

Dokumen ini mengikuti implementasi backend saat ini, bukan rancangan fitur mendatang. Sumber utama: router, middleware, controller, service, dan utilitas di `backend/`. Kontrak upload, daftar file, metadata, download hasil dekripsi, konfigurasi penyimpanan, dan pengukuran waktu AES dijelaskan di [Panduan API File](FILES.md).

## 1. Status implementasi

| Fitur | Status |
| --- | --- |
| Register pengguna | Tersedia; selalu default role `patient` |
| Login dengan username dan password | Tersedia |
| Penerbitan JWT saat login | Tersedia |
| Middleware untuk verifikasi JWT / endpoint terproteksi | Tersedia; `AuthMiddleware` membaca cookie `access_token` |
| Profil pengguna / endpoint `/me` | Tersedia; `GET /api/users/me` mengembalikan identitas dan role dari database |
| Logout browser | Tersedia; `POST /api/users/logout` menghapus cookie HttpOnly, lihat bagian 5.1 |
| Refresh token dan revokasi JWT server-side | Belum tersedia |
| Upload, metadata, dan download file tiga cipher | Tersedia; upload baru AES/DES/RC4 all-or-nothing, download `?algorithm=aes|des|rc4`; lihat bagian 11 |
| Lampiran pasien dan akses dokter | Tersedia; file memiliki pasien subjek terpisah dari pengunggah; `GET /api/files` mengikuti role, lihat bagian 12 |
| Create/baca profil pasien sendiri | Tersedia; `POST` dan `GET /api/patients/me/profile`, NIK/BPJS record baru terenkripsi AES + DES + RC4; GET mengembalikan nilai terdekripsi hanya kepada pasien pemilik; lihat [kontrak profil pasien](#9-profil-pasien) |
| Baca data KTP pasien sendiri | Tersedia; `GET /api/patients/me/ktp`; lihat [kontrak KTP](#10-ktp-pasien) |
| Baca foto KTP pasien sendiri | Tersedia; `GET /api/patients/me/ktp/photo`; lihat bagian 10.4 |
| Create KTP dan upload foto | Tersedia; `POST /api/patients/me/ktp`, multipart data JSON + photo; lihat bagian 10.7 |
| RC4 / integrasi DES | DES dan RC4 tersedia untuk profil dan pembacaan KTP; pipeline file generik juga memakai ketiga cipher untuk upload baru |
| CORS untuk akses lintas origin dari browser | Tersedia |

**FrontEnd dapat mengintegrasikan register, login, upload file, daftar lampiran sesuai role/pasien subjek, metadata per ID, dan download hasil dekripsi AES/DES/RC4.** Gunakan hanya endpoint yang tercantum; pemilihan cipher file melalui query algorithm pada endpoint dekripsi. Role `patient`/`doctor`, kontrak `/me`, migrasi, dan penyediaan dokter dijelaskan di [Panduan Role](ROLES.md).

## 2. Base URL dan konfigurasi

```text
http://localhost:<SERVER_PORT>/api
```

Port diambil dari environment variable `SERVER_PORT`. Tidak ada port default yang ditetapkan di kode. Contoh dalam dokumen ini menggunakan `SERVER_PORT=8080`, sehingga base URL-nya adalah `http://localhost:8080/api`; sesuaikan dengan konfigurasi backend yang dijalankan.

Semua endpoint yang tersedia:

| Method | Path lengkap | Autentikasi | Fungsi |
| --- | --- | --- | --- |
| `POST` | `/api/users/register` | Tidak diperlukan | Membuat pengguna |
| `POST` | `/api/users/login` | Tidak diperlukan | Mengatur cookie JWT `access_token` |
| `POST` | `/api/users/logout` | Token valid tidak diwajibkan | Menghapus cookie `access_token`; idempotent, bukan revokasi JWT |
| `GET` | `/api/users/me` | Cookie `access_token` | Identitas dan role terbaru user login; berbeda dari profil pasien |
| `POST` | `/api/patients/me/profile` | Cookie `access_token`, role `patient` | Membuat profil pasien sendiri; kontrak di bagian 9 |
| `GET` | `/api/patients/me/profile` | Cookie `access_token`, role `patient` | Membaca profil sendiri beserta NIK/BPJS terdekripsi; kontrak di bagian 9 |
| `POST` | `/api/patients/me/ktp` | Cookie `access_token`, role `patient` | Membuat data KTP dan upload foto; kontrak di bagian 10.7 |
| `GET` | `/api/patients/me/ktp` | Cookie `access_token`, role `patient` | Membaca data KTP sendiri; kontrak di bagian 10 |
| `GET` | `/api/patients/me/ktp/photo` | Cookie `access_token`, role `patient` | Mengambil bytes foto KTP sendiri setelah dekripsi; kontrak di bagian 10.4 |
| `POST` | `/api/files` | Cookie `access_token`, role patient | Upload untuk profil pasien sendiri; tiga ciphertext AES/DES/RC4 |
| `GET` | `/api/doctor/patients` | Cookie `access_token`, role doctor | Daftar ringkas pasien yang sudah punya profil |
| `POST` | `/api/doctor/patients/:patient_profile_id/files` | Cookie `access_token`, role doctor | Unggah laporan/lampiran untuk pasien terkait |
| `GET` | `/api/doctor/patients/:patient_profile_id/files` | Cookie `access_token`, role doctor | Daftar lampiran yang terhubung ke pasien terkait |
| `GET` | `/api/files` | Cookie `access_token` | Metadata lampiran sesuai role/subject, ID DESC, tanpa pagination/filter |
| `GET` | `/api/files/:id` | Cookie `access_token` | Metadata lampiran yang boleh diakses menurut role/subject |
| `POST` | `/api/files/:id/decrypt` | Cookie `access_token` | Dekripsi/download byte asli; pilih algorithm aes/des/rc4, default aes |

Tidak ada prefix versi seperti `/v1`.

`GET /api/files` mengembalikan `200 OK` dengan `{"files":[...]}` (kosong: `{"files":[]}`) dan `Cache-Control: no-store`. Setiap item memakai DTO metadata yang sama dengan `GET /api/files/:id`: `id`, `original_name`, `original_size`, `original_type`, `algorithm` (default AES), `algorithms`, `cipher_set_version`, serta enam field timing AES/DES/RC4; lihat bagian 11. Urutan **ID DESC** berarti record terbaru diinsert terlebih dahulu. Identitas berasal dari JWT, role terbaru dari database, dan akses mengikuti pasien subjek, bukan sekadar pengunggah (bagian 12). Query `user_id`/`patient_profile_id` tidak mengubah scope akses. Endpoint tidak menerima pagination/filter, tidak membaca disk atau mengukur ulang timing, dan tetap menyertakan record historis metadata-only. Response tidak berisi path disk atau key. Error autentikasi `401` mengikuti middleware yang sama (lihat bagian 5); kegagalan pemrosesan menghasilkan `500` dengan `{"message":"Failed to process file"}`. Contoh fetch dan curl cookie jar ada di [Panduan API File](FILES.md).

### Header request

```http
Content-Type: application/json
Accept: application/json
```

Untuk **register dan login**, kirim body JSON, bukan `FormData` atau form URL-encoded. Kedua endpoint tidak memerlukan header `Authorization`. Endpoint file menggunakan cookie, bukan Bearer token. Upload memakai `FormData` dengan field `file`; **jangan mengatur `Content-Type` secara manual** karena browser harus menambahkan boundary multipart. Dekripsi sukses mengembalikan binary, bukan JSON; lihat [Panduan API File](FILES.md).

### Menjalankan backend untuk integrasi lokal

Backend menggunakan Go (versi pada `backend/go.mod`: `1.26.0`) dan PostgreSQL. Buat file `backend/.env` secara lokal dengan konfigurasi berikut:

```dotenv
SERVER_PORT=8080
DATABASE_URL=postgres://app_user:local_password@localhost:5432/app_db?sslmode=disable
JWT_SECRET=ganti-dengan-secret-acak-yang-kuat
```

Nilai di atas hanya contoh; sesuaikan kredensial database. `sslmode=disable` hanya untuk contoh lingkungan lokal. Jangan commit kredensial atau secret ke repository.

Tambahkan juga **`FILE_ENCRYPTION_KEY` yang wajib**: jalankan `openssl rand -hex 32` sekali, lalu simpan output 64 karakter hex (32 byte acak) sebagai nilai `FILE_ENCRYPTION_KEY` di `backend/.env`. Jangan membuat ulang key pada setiap startup; file lama membutuhkan key yang sama untuk didekripsi. Backup key secara terpisah dan aman. `FILE_STORAGE_DIR` opsional, default `storage/uploads` relatif terhadap working directory backend. Gunakan storage/volume persisten, jangan public static hosting. Detail konfigurasi dan pemulihan ada di [Panduan API File](FILES.md#1-konfigurasi-dan-persistensi).

Jalankan dari direktori `backend/`:

```sh
go run .
```

Backend memanggil `godotenv.Load()` dan akan berhenti jika file `.env` gagal dimuat. Saat startup, backend juga menghubungkan database dan menjalankan `AutoMigrate` model `User` serta `File`, termasuk kolom timing AES dengan default `0` untuk record historis. Startup gagal jika `FILE_ENCRYPTION_KEY` tidak valid atau direktori penyimpanan gagal disiapkan.

`DATABASE_URL`, `JWT_SECRET`, dan `FILE_ENCRYPTION_KEY` hanya untuk backend. **Jangan memasukkan secret, key enkripsi file, atau kredensial database ke environment FrontEnd yang dibundel ke browser.** Key enkripsi tidak disimpan dalam database atau dikirim ke browser.

## 3. Register

### Request

```http
POST /api/users/register
```

```json
{
  "name": "Nopal",
  "username": "nopal",
  "password": "contoh-password-kuat"
}
```

| Field | Tipe | Wajib | Aturan yang sudah diimplementasikan |
| --- | --- | --- | --- |
| `name` | `string` | Ya | Tidak boleh string kosong |
| `username` | `string` | Ya | Tidak boleh string kosong; dicek apakah sudah dipakai |
| `password` | `string` | Ya | Tidak boleh string kosong; disimpan sebagai hash bcrypt |

Catatan validasi saat ini:

- Belum ada validasi email, panjang minimum password, atau pola karakter username.
- Backend tidak melakukan trim atau normalisasi huruf besar/kecil. Jangan mengasumsikan whitespace dibuang atau username dibuat lowercase.
- String yang hanya berisi spasi belum ditolak oleh aturan `required`; FrontEnd sebaiknya memvalidasi input nama/username sebelum dikirim.
- Bcrypt membatasi password sampai **72 byte**, bukan 72 karakter. Password yang melebihi batas itu gagal di-hash dan saat ini menghasilkan `500`, bukan `400`.
- FrontEnd boleh menerapkan validasi UX tambahan, tetapi itu bukan jaminan validasi server. Jangan melakukan trim password secara diam-diam.

### Response sukses — `201 Created`

```json
{
  "message": "User registered successfully"
}
```

Register **tidak** mengembalikan token, ID, atau objek pengguna. Setelah sukses, arahkan pengguna ke login; jangan menandai pengguna sudah terautentikasi hanya berdasarkan response register.

### Response error

| Status | Kondisi | `message` |
| --- | --- | --- |
| `400 Bad Request` | JSON tidak valid, tipe field salah, atau field wajib kosong/tidak dikirim | Detail error binding/validasi dari Gin; teksnya bergantung pada input |
| `409 Conflict` | Database menolak username yang sudah terdaftar | `username already exist` |
| `500 Internal Server Error` | Hash password atau penyimpanan ke database gagal | Pesan error dari operasi yang gagal |

Contoh username sudah dipakai:

```json
{
  "message": "username already exist"
}
```

Contoh validasi ketika hanya `name` yang kosong:

```json
{
  "message": "Key: 'RegisterRequest.Name' Error:Field validation for 'Name' failed on the 'required' tag"
}
```

Contoh tersebut bukan teks tetap untuk semua `400`. Gunakan status HTTP untuk menentukan perilaku UI; jangan parsing string validasi Gin. Benturan unique constraint PostgreSQL saat penyimpanan dipetakan ke `409`, termasuk jika dua permintaan mendaftarkan username yang sama bersamaan.

### Contoh curl

Dengan asumsi `SERVER_PORT=8080`:

```sh
curl -i -X POST http://localhost:8080/api/users/register \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  --data '{"name":"Nopal","username":"nopal","password":"contoh-password-kuat"}'
```

## 4. Login

### Request

```http
POST /api/users/login
```

```json
{
  "username": "nopal",
  "password": "contoh-password-kuat"
}
```

| Field | Tipe | Wajib | Aturan |
| --- | --- | --- | --- |
| `username` | `string` | Ya | Tidak boleh string kosong |
| `password` | `string` | Ya | Tidak boleh string kosong |

Login menggunakan **username**, bukan nama atau email.

### Response sukses — `200 OK`

```json
{
  "message": "Login successful"
}
```

JWT **tidak dikembalikan dalam JSON**. Response mengatur cookie `access_token` dengan `HttpOnly`, `SameSite=Lax`, `Path=/`, dan `Max-Age=86400` (24 jam), tanpa atribut Domain eksplisit. Implementasi saat ini memakai `Secure=false`; jangan menganggap konfigurasi lokal ini sudah siap produksi. Response tidak menyertakan objek pengguna, `token`, `refresh_token`, `token_type`, atau `expires_in`.

Browser menyimpan/mengirim cookie; JavaScript tidak dapat membacanya melalui `document.cookie`. Untuk login dan request file lintas origin, gunakan `credentials: "include"`.

### Response error

| Status | Kondisi | `message` |
| --- | --- | --- |
| `400 Bad Request` | JSON tidak valid, tipe field salah, atau field wajib kosong/tidak dikirim | `Invalid request` |
| `401 Unauthorized` | Username tidak ditemukan atau password tidak cocok | `Invalid password or username` |
| `401 Unauthorized` | Service gagal membaca pengguna dari database atau gagal membuat token | `Invalid password or username` |

```json
{
  "message": "Invalid request"
}
```

```json
{
  "message": "Invalid password or username"
}
```

Controller login saat ini memetakan seluruh error service ke `401`. Karena itu, `401` tidak selalu bisa dipastikan sebagai kesalahan input pengguna saja.

### Contoh curl

```sh
curl -i -c cookies.txt -X POST http://localhost:8080/api/users/login \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  --data '{"username":"nopal","password":"contoh-password-kuat"}'
```

## 5. JWT dan alur autentikasi FrontEnd

Token ditandatangani dengan **HS256**, menggunakan `JWT_SECRET` milik backend, dan memiliki masa berlaku **24 jam sejak dibuat**.

Payload yang dibuat backend:

```json
{
  "userId": 1,
  "username": "nopal",
  "exp": 1791158400
}
```

Angka di atas hanya contoh struktur internal JWT, bukan response login. `userId` (camelCase) adalah ID database, sedangkan `exp` adalah Unix timestamp dalam **detik**, bukan milidetik. FrontEnd tidak perlu membaca JWT untuk memanggil endpoint file.

Alur yang dapat diterapkan sekarang:

1. Register melalui `/api/users/register`.
2. Setelah `201`, tampilkan informasi sukses dan arahkan ke halaman login.
3. Login melalui `/api/users/login` dengan `credentials: "include"` jika lintas origin.
4. Setelah `200`, browser menyimpan cookie `HttpOnly`; panggil `GET /api/users/me` dengan cookie untuk membaca `id`, `name`, `username`, dan `role` sebelum memilih tampilan. Jangan membaca `result.token` atau menyimpan JWT di `localStorage`/`sessionStorage`. Role tidak disimpan dalam JWT; `/me` membaca nilai terbaru dari database.
5. Kirim cookie pada request file dengan `credentials: "include"`; `AuthMiddleware` memvalidasi JWT dan meneruskan ID pengguna. Service membaca role terbaru dari database; daftar, metadata, dan dekripsi mengikuti akses pasien subjek/dokter serta aturan legacy pada bagian 12, bukan hanya ID pengunggah.
6. Jika request terproteksi menghasilkan `401`, minta pengguna login lagi; belum ada endpoint refresh.

Middleware membaca **cookie `access_token`**, bukan header `Authorization: Bearer ...`. Cookie hilang menghasilkan `401` dengan `message: "Missing authentication cookie"`; JWT invalid/kedaluwarsa menghasilkan `"Invalid or expired token"`. Claim ID tidak valid dapat menghasilkan `"Unauthorized: user_id missing"`; controller file juga dapat mengembalikan `"Unauthorized"`.

Logout browser tersedia melalui `POST /api/users/logout` di bawah. Menghapus state UI saja **tidak** menghapus cookie HttpOnly; frontend perlu memanggil endpoint logout. Revokasi JWT server-side belum tersedia.

### 5.1. Logout — `POST /api/users/logout`

Tidak memerlukan body atau Content-Type request. Tidak dipasang middleware autentikasi agar cookie tetap dapat dibersihkan walaupun token invalid/kedaluwarsa atau cookie sudah hilang. Berlaku untuk kedua role dan tidak melakukan query database.

Respons **`200 OK`**, termasuk jika dipanggil berulang atau tanpa cookie:

```json
{
  "message": "Logout successful"
}
```

Backend mengirim `Set-Cookie` penghapusan `access_token` dengan nilai kosong dan Max-Age negatif (header `Max-Age=0`), menggunakan scope login yang sama: Path `/`, tanpa Domain eksplisit, HttpOnly, SameSite=Lax. Secure saat ini `false` mengikuti konfigurasi login lokal; ini bukan konfigurasi cookie produksi yang sudah diperkeras. Respons memakai `Cache-Control: no-store`.

Contoh frontend:

```js
const response = await fetch(`${API_BASE_URL}/users/logout`, {
  method: "POST",
  credentials: "include"
});
if (!response.ok) {
  throw new Error("Logout gagal; coba lagi.");
}
// Setelah browser memproses penghapusan cookie, bersihkan state user dan arahkan ke login.
```

Browser memproses penghapusan cookie; JavaScript tidak perlu dan tidak bisa menghapus cookie HttpOnly secara langsung. Setelah cookie terhapus, request endpoint terproteksi tanpa cookie mendapat `401`. `GET /api/users/logout` tidak melakukan logout.

**Batas penting:** endpoint ini menghapus cookie pada browser tersebut, bukan mencabut JWT di backend atau logout semua perangkat. Salinan token yang sudah dicuri/disimpan di tempat lain tetap dapat dipakai sampai kedaluwarsa (saat ini 24 jam sejak login). Belum ada denylist/session revocation; jangan mengklaim logout ini membatalkan semua salinan token. Request yang sudah berjalan juga tidak otomatis dibatalkan.

## 6. Contoh integrasi JavaScript dengan fetch

Contoh ini menggunakan path relatif `/api`. Artinya FrontEnd harus berada pada origin yang sama dengan backend atau menggunakan proxy development yang meneruskan `/api` ke backend. Untuk akses langsung dari `http://localhost:5173`, ubah base URL menjadi `http://localhost:8080/api` (sesuaikan port); `credentials: "include"` diperlukan agar cookie login disimpan dan dikirim lintas origin.

```js
const API_BASE_URL = "/api";

class ApiError extends Error {
  constructor(status, message) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

async function postJson(path, body) {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
    },
    body: JSON.stringify(body),
  });

  // Route yang tidak tersedia atau error dari proxy bisa bukan JSON.
  const data = await response.json().catch(() => null);

  if (!response.ok) {
    throw new ApiError(
      response.status,
      data?.message ?? `Request gagal (HTTP ${response.status})`,
    );
  }

  if (data === null) {
    throw new Error("Response sukses tidak berisi JSON yang valid");
  }

  return data;
}

function register({ name, username, password }) {
  return postJson("/users/register", { name, username, password });
}

function login({ username, password }) {
  return postJson("/users/login", { username, password });
}

// State UI saja; autentikasi backend tetap ditentukan oleh cookie.

async function submitLogin(username, password) {
  try {
    await login({ username, password });
    // Update state UI dan arahkan ke halaman setelah login di sini.
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      // Tampilkan pesan kegagalan login yang ramah pengguna.
    } else {
      // Tangani validasi request, server error, atau kegagalan jaringan.
    }
    throw error;
  }
}

```

`fetch` tidak otomatis melempar error untuk HTTP `400`, `401`, `409`, atau `500`; karena itu helper memeriksa `response.ok`. Error jaringan/CORS biasanya tidak menyediakan status HTTP yang dapat dibaca, sehingga jangan menyamakannya dengan `401`.

### Bentuk data untuk TypeScript

```ts
type RegisterRequest = {
  name: string;
  username: string;
  password: string;
};

type LoginRequest = {
  username: string;
  password: string;
};

type RegisterResponse = {
  message: string;
};

type LoginResponse = {
  message: string;
};

type ErrorResponse = {
  message: string;
};
```

Tipe error tersebut berlaku untuk response JSON dari controller autentikasi, bukan jaminan untuk semua response server/proxy atau path yang tidak terdaftar.

## 7. CORS dan checklist integrasi

Router memakai `gin.Default()` dengan middleware CORS. Origin yang diizinkan saat ini hanya **`http://localhost:5173`**, dengan `AllowCredentials: true`. Method yang diizinkan: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS`; ini bukan berarti semuanya memiliki endpoint. Header request yang diizinkan: `Origin`, `Content-Type`, `Authorization`. Header response yang diekspos ke JavaScript: `Content-Length`, `Content-Disposition`, `X-Decryption-Time-Ns`, `X-Decryption-Algorithm`.

Untuk development, gunakan salah satu pendekatan:

- Konfigurasikan proxy dev server FrontEnd agar request relatif `/api/...` diteruskan ke `http://localhost:<SERVER_PORT>/api/...`, **tanpa menghapus prefix `/api`**.
- Akses backend langsung dari `http://localhost:5173` menggunakan base URL backend dan `credentials: "include"` pada login maupun seluruh request file. Origin lain (termasuk `http://127.0.0.1:5173`) belum diizinkan oleh konfigurasi saat ini.

Jangan gunakan `mode: "no-cors"` sebagai solusi; response menjadi opaque dan isinya tidak dapat dibaca. Request `curl` yang berhasil belum membuktikan bahwa akses lintas origin dari browser sudah bekerja. `SameSite=Lax` juga membatasi cookie pada skenario cross-site; `credentials: "include"` tidak melewati aturan cookie browser. CORS lokal ini bukan jaminan kesiapan produksi atau perlindungan CSRF.

Checklist FrontEnd:

- [ ] Base URL atau target proxy sesuai `SERVER_PORT` backend.
- [ ] Register/login dikirim sebagai JSON; upload sebagai `FormData` field `file` tanpa `Content-Type` manual.
- [ ] Form login memakai `username`, bukan `email`.
- [ ] Register sukses ditangani sebagai `201`, bukan hanya `200`.
- [ ] Register tidak dianggap otomatis login.
- [ ] Error ditangani berdasarkan status HTTP; pesan internal `500` sebaiknya tidak ditampilkan mentah ke pengguna.
- [ ] Error jaringan/CORS ditangani terpisah dari kegagalan kredensial.
- [ ] Login dan request file memakai `credentials: "include"` untuk akses lintas origin; tidak mengharapkan `result.token`.
- [ ] Token/key tidak dicetak ke log atau dibagikan; komunikasi produksi menggunakan HTTPS (cookie saat ini belum `Secure`).
- [ ] Download dekripsi sukses dibaca dengan `response.blob()`; error JSON ditangani berdasarkan status.
- [ ] Timing nanodetik dibagi `1e6` untuk milidetik; pahami cakupan pengukuran di [Panduan API File](FILES.md#4-arti-dan-batasan-timing).
- [ ] Daftar file memakai `GET /api/files` tanpa pagination/filter; owner ditentukan cookie, bukan query `user_id`.
- [ ] UI profil/RC4/DES tidak memanggil endpoint yang belum tersedia.

## 8. Referensi implementasi

Path relatif terhadap root project:

- `backend/routers/router.go` — prefix `/api` dan setup router.
- `backend/routers/userRouter.go` — registrasi route register/login.
- `backend/controllers/authController.go` — request, validasi binding, status HTTP, dan response JSON.
- `backend/services/authService.go` — pengecekan username, bcrypt, dan alur login.
- `backend/repositories/userRepository.go` — pencarian username dan penyimpanan pengguna.
- `backend/utils/jwt.go` — algoritma, claims, dan masa berlaku JWT.
- `backend/middlewares/authMiddleware.go` — validasi cookie JWT pada route terproteksi.
- `backend/routers/fileRouter.go` dan `backend/controllers/fileController.go` — endpoint file, DTO metadata, header download, dan status error.
- `backend/services/fileService.go` dan `backend/repositories/fileRepository.go` — persistensi disk/database, kepemilikan, dan timing.
- `backend/utils/aes_file.go` — envelope AES1 dan derivasi key; AES buatan tangan untuk pembelajaran, bukan keamanan produksi.
- `backend/main.go` dan `backend/config/database.go` — environment, startup, dan database.
- `backend/models/user.go` dan `backend/models/file.go` — model database, bukan kontrak response API.

Jika backend berubah, perbarui dokumen ini berdasarkan router dan controller yang benar-benar terpasang.

## 9. Profil pasien

Kontrak berikut mengikuti endpoint yang sudah dipasang di `backend/routers/patientRouter.go`. Endpoint ini berbeda dari `GET /api/users/me`, yang hanya mengembalikan identitas akun dan role. Belum ada endpoint update/delete profil. Kontrak create/upload dan pembacaan KTP tersedia di bagian 10.

### Autentikasi dan ownership

- Keduanya menggunakan cookie HttpOnly `access_token`, bukan Bearer token. Untuk fetch lintas origin gunakan `credentials: "include"`.
- Backend mengambil user ID dari JWT tervalidasi lalu membaca role terbaru dari database. Hanya role `patient` yang diizinkan; dokter mendapat `403` pada endpoint profil pasien sendiri ini.
- Tidak ada pemilihan pemilik melalui body atau URL. Query `user_id`/`patient_profile_id` pada GET tidak memilih profil lain; query database tetap dibatasi user yang login.
- Controller mengirim `Cache-Control: no-store`. Jangan menyimpan NIK/BPJS atau respons profil di log, localStorage, maupun sessionStorage. Di luar development lokal gunakan HTTPS.
- Semua nomor, nama, dan ID pada contoh berikut adalah data sintetis, bukan hasil pengukuran atau data pasien asli.

### 9.1. Create profil — `POST /api/patients/me/profile`

Kirim satu JSON object, maksimal **4 KiB**, dengan `Content-Type: application/json`:

```json
{
  "weight_kg": 65.5,
  "height_cm": 170,
  "age": 21,
  "nik": "0000000000000001",
  "bpjs_number": "0000000000001"
}
```

| Field | Tipe JSON | Validasi |
| --- | --- | --- |
| `weight_kg` | number | Positif, maksimal 9999.99, maksimal dua angka desimal; satuan kg |
| `height_cm` | number | Positif, maksimal 9999.99, maksimal dua angka desimal; satuan cm |
| `age` | integer | Wajib hadir, bukan null, 0–150; satuan tahun |
| `nik` | string | Tepat 16 digit ASCII; jangan kirim sebagai number |
| `bpjs_number` | string | Tepat 13 digit ASCII; jangan kirim sebagai number |

Seluruh field wajib. Nol di depan nomor identitas dipertahankan. Field tambahan, termasuk `user_id`, `role`, kolom ciphertext, atau nested KTP ditolak. Batas berat/tinggi mengikuti kapasitas penyimpanan, bukan validasi klinis; validasi nomor hanya memeriksa format, bukan membuktikan identitas/keanggotaan resmi.

Respons sukses — **`201 Created`**:

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
    "active": ["aes", "des", "rc4"],
    "pending": []
  }
}
```

Respons create tidak mengirim NIK/BPJS, ciphertext, association KTP, password hash, atau kunci. NIK/BPJS record baru disimpan sebagai tiga representasi AES/DES/RC4; setiap hasil diverifikasi round-trip sebelum insert. Berat, tinggi, dan umur tetap plaintext sesuai keputusan saat ini. Nilai `active` **bukan klaim seluruh profil terenkripsi atau aplikasi aman untuk produksi**.

| Status error POST | Kondisi |
| --- | --- |
| `400` | JSON/field/tipe/format input tidak valid, termasuk field tambahan atau lebih dari satu JSON object |
| `401` | Cookie/token tidak valid, ID kosong, atau user sudah dihapus |
| `403` | Role terbaru bukan pasien |
| `409` | User sudah mempunyai profil; endpoint tidak overwrite/upsert |
| `413` | Body melebihi 4 KiB |
| `415` | Content-Type bukan application/json |
| `500` | Gangguan database/kriptografi; pesan generik |

Semua respons error memakai bentuk `{"message":"..."}`. Jangan membuat profil kedua untuk memperbarui data: endpoint update belum tersedia.

### 9.2. Baca profil sendiri — `GET /api/patients/me/profile`

Tidak perlu body, Content-Type, atau ID pasien. Kirim cookie login.

Respons sukses — **`200 OK`**:

```json
{
  "profile": {
    "id": 12,
    "user_id": 7,
    "weight_kg": 65.5,
    "height_cm": 170,
    "age": 21,
    "nik": "0000000000000001",
    "bpjs_number": "0000000000001"
  }
}
```

NIK/BPJS dikembalikan sebagai string setelah dekripsi. DTO tidak menyertakan ciphertext, association KTP, role internal, password hash, atau kunci. Umur adalah nilai tersimpan, tidak otomatis bertambah.

Sebelum respons dikirim, AES dan DES wajib lolos autentikasi/dekripsi dan menghasilkan nilai identik; RC4 yang tersimpan juga wajib menghasilkan nilai sama. Format NIK/BPJS hasil dekripsi diperiksa kembali. Jika salah satu gagal atau berbeda, tidak ada profil atau data sebagian yang dikirim.

Kompatibilitas record lama: hanya bila **kedua** kolom RC4 NULL, GET membaca dan mencocokkan AES/DES tanpa mengklaim record sudah dilindungi RC4. Satu kolom RC4 NULL saja atau bytes RC4 kosong/rusak ditolak. NULL merupakan jalur kompatibilitas, bukan bukti terautentikasi bahwa record benar-benar lama. GET tidak melakukan backfill atau perubahan database.

| Status error GET | Kondisi | Contoh message |
| --- | --- | --- |
| `401` | Cookie/token tidak valid atau user sudah tidak ada | Pesan middleware autentikasi atau `Unauthorized` |
| `403` | Role terbaru bukan pasien | `only patients can access their own profile` |
| `404` | Pasien belum mempunyai profil | `Patient profile not found` |
| `500` | Query/dekripsi gagal atau representasi tidak konsisten | `Unable to read patient profile` |

Error tetap berbentuk `{"message":"..."}`, tanpa data sebagian. Pada `404`, frontend dapat menawarkan form create profil; jangan menganggapnya kegagalan login.

Pemeriksaan cipher tidak mengautentikasi berat/tinggi/umur yang masih plaintext, tidak memverifikasi identitas resmi, dan tidak membuktikan kebal manipulasi administrator. DES/RC4 serta implementasi edukasi bukan rekomendasi keamanan medis produksi.

Detail schema, key, kompatibilitas data lama, dan pengujian ada di [PATIENT_PROFILES.md](PATIENT_PROFILES.md). Kontrak request/response profil dipusatkan di bagian ini.

## 10. KTP pasien

### 10.1. Status kelima endpoint pasien

| Endpoint | Fungsi | Status implementasi |
| --- | --- | --- |
| `POST /api/patients/me/profile` | Pasien membuat profil sendiri | Tersedia |
| `GET /api/patients/me/profile` | Pasien membaca profil sendiri | Tersedia |
| `POST /api/patients/me/ktp` | Pasien mengirim data KTP dan foto | Tersedia |
| `GET /api/patients/me/ktp` | Pasien membaca data KTP sendiri | Tersedia |
| `GET /api/patients/me/ktp/photo` | Pasien mengambil foto KTP setelah dekripsi | Tersedia |

Alur backend tersedia: buat profil → POST data/foto KTP → GET data/foto sendiri. Kedua GET membaca record yang benar-benar tersimpan; tanpa profil/record KTP, responsnya `404`. Contoh dan fixture tes memakai data sintetis; jangan menampilkan data contoh sebagai record pasien sungguhan.

### 10.2. Autentikasi, ownership, dan integritas

- Kedua GET KTP memakai cookie HttpOnly `access_token`, bukan Bearer token. Gunakan `credentials: "include"` untuk fetch lintas origin. Tidak perlu body, Content-Type request, atau ID pasien.
- Backend membaca role terbaru dari database melalui pemeriksaan profil yang sudah ada. Hanya role `patient` boleh memakai endpoint `/me/ktp`; dokter mendapat `403`.
- Query KTP menghubungkan `patient_ktps.patient_profile_id` ke `patient_profiles.id` dan membatasi `patient_profiles.user_id` berdasarkan JWT. Query `user_id`, `patient_profile_id`, atau `ktp_id` kiriman klien tidak memilih pemilik lain.
- NIK KTP didekripsi dengan ketiga cipher dan wajib sama dengan NIK profil pasien. Profil juga harus lolos pemeriksaan dekripsi yang dijelaskan di bagian 9.2.
- Setiap field teks/foto yang dikirim wajib lolos autentikasi AES/DES/RC4 dan menghasilkan plaintext identik. Tidak ada fallback KTP jika salah satu cipher hilang/rusak. Kompatibilitas RC4 NULL pada profil lama tidak berlaku pada kolom KTP.
- Respons controller memakai `Cache-Control: no-store` dan `X-Content-Type-Options: nosniff`. Jangan menyimpan hasilnya di log atau browser storage. Gunakan HTTPS di luar development lokal.
- GET tidak melakukan update, backfill, atau menulis plaintext ke filesystem.

### 10.3. Baca data — `GET /api/patients/me/ktp`

Respons **`200 OK`**, `Content-Type: application/json` (seluruh data di bawah sintetis):

```json
{
  "ktp": {
    "id": 3,
    "patient_profile_id": 12,
    "nik": "0000000000000001",
    "full_name": "Pasien Sintetis",
    "birth_place": "Kota Contoh",
    "birth_date": "2005-01-01",
    "gender": "Laki-laki",
    "blood_type": "O",
    "address": "Jalan Contoh 1",
    "rt": "001",
    "rw": "002",
    "village": "Desa Contoh",
    "district": "Kecamatan Contoh",
    "religion": "Islam",
    "marital_status": "Belum Kawin",
    "occupation": "Pelajar",
    "citizenship": "WNI"
  }
}
```

`id` dan `patient_profile_id` adalah ID numerik. Semua field lainnya string; NIK/RT/RW tidak dikonversi menjadi angka sehingga nol di depan tetap ada. Alamat, RT, RW, kelurahan/desa (`village`), dan kecamatan (`district`) terpisah.

Field teks dibatasi 16 KiB plaintext per field dan wajib UTF-8 valid. Endpoint read tidak membuktikan kebenaran identitas, format pilihan resmi, atau keaslian dokumen.

Respons tidak berisi foto/Base64, ciphertext, kunci, atau association model. Query metadata tidak memuat tiga blob foto dari database, dan endpoint ini tidak memeriksa integritas foto. Foto diperiksa terpisah oleh endpoint bagian 10.4. Bila satu field teks gagal, tidak ada data KTP sebagian yang dikirim.

### 10.4. Baca foto — `GET /api/patients/me/ktp/photo`

Kirim cookie login, tanpa body, ID pasien, atau Content-Type request. Hanya pasien pemilik yang boleh mengakses; role terbaru dibaca dari database. Parameter query tidak dapat memilih pasien lain.

Respons **`200 OK` berupa bytes gambar asli**, bukan JSON/Base64/path:

```http
Content-Type: image/png
Content-Disposition: inline; filename="ktp.png"
Cache-Control: no-store
X-Content-Type-Options: nosniff
```

Untuk JPEG, Content-Type adalah `image/jpeg` dan filename `ktp.jpg`. Nama file tetap, tanpa nama pasien atau NIK.

- Backend membaca kolom `photo_aes`, `photo_des`, `photo_rc4` langsung dari PostgreSQL melalui query owner-scoped. Tidak memakai filesystem, folder static, atau URL publik.
- Profil dan NIK KTP harus lolos pemeriksaan ownership/kecocokan; setiap ciphertext foto wajib lolos autentikasi dan ketiganya menghasilkan bytes identik. Tidak ada fallback jika salah satu hilang/rusak.
- Bytes hasil dikirim tanpa re-encode, termasuk metadata gambar asli. Pemeriksaan header memakai decoder PNG/JPEG; bukan OCR, verifikasi keaslian KTP, atau validasi penuh seluruh isi gambar.
- Batas pembacaan: maksimal **8 MiB plaintext**, maksimal **25 juta pixel** berdasarkan dimensi header. Ukuran envelope dibatasi sebelum alokasi dekripsi.
- Query foto hanya memuat ID, relasi profil, NIK, dan ciphertext foto; tidak membaca/mendekripsi seluruh field teks KTP. Endpoint data teks tetap tidak memuat blob foto.
- Semua pemeriksaan selesai sebelum pengiriman binary. Jika gagal, respons JSON generik dikirim tanpa bytes foto sebagian. Foto kosong/rusak pada record yang sudah ada menghasilkan `500`, bukan `404`.
- Frontend mengecek `response.ok` terlebih dahulu. Sukses dibaca dengan `response.blob()` atau `arrayBuffer()`; error dibaca sebagai JSON. Jangan menyimpan foto ke browser storage atau log.

POST `/api/patients/me/ktp` pada bagian 10.7 menyediakan upload yang dapat dibaca kembali lewat endpoint ini. Foto pada tes merupakan data sintetis.

### 10.5. Error kedua GET KTP

| Status | Kondisi | Message dari controller |
| --- | --- | --- |
| `401` | User tidak terautentikasi atau sudah dihapus | `Unauthorized`, atau pesan middleware cookie/token |
| `403` | Role terbaru bukan pasien | `Only patients can access their own KTP` |
| `404` | Pasien belum mempunyai profil | `Patient profile not found` |
| `404` | Profil ada tetapi belum mempunyai record KTP | `Patient KTP not found` |
| `500` | Gangguan database, dekripsi/integritas gagal, NIK berbeda, representasi hilang/tidak cocok, atau teks/foto tidak memenuhi batas pembacaan | `Unable to read patient KTP` |

Semua error berbentuk `{"message":"..."}` tanpa data KTP atau bytes foto sebagian. Kondisi foto hanya diperiksa endpoint foto, bukan endpoint data teks.

### 10.6. Batas implementasi dan verifikasi

Endpoint read, tes router/service dengan cipher sungguhan, dan tes SQL DryRun tersedia. Tes memakai record KTP dan foto sintetis; mencakup kesamaan bytes PNG/JPEG, penolakan akses tidak sah, ciphertext rusak/tidak cocok, serta batas ukuran/dimensi. Tes juga mencakup POST multipart → GET data/foto dengan middleware dan kriptografi sungguhan, tetapi repository masih in-memory/DryRun, bukan PostgreSQL nyata. **Kelima endpoint bagian 10.1 sudah diimplementasikan**; ini bukan klaim seluruh aplikasi atau frontend selesai.

Integritas kriptografis tidak membuktikan kebenaran data identitas atau ketahanan terhadap seluruh manipulasi administrator. DES/RC4 tetap untuk assignment, bukan rekomendasi sistem medis produksi. Detail schema ada di [PATIENT_KTP.md](PATIENT_KTP.md).

### 10.7. Create data KTP dan upload foto — `POST /api/patients/me/ktp`

**Prasyarat:** login sebagai pasien dan sudah mempunyai profil melalui POST `/api/patients/me/profile`. NIK diambil dari profil yang lolos pemeriksaan dekripsi; jangan mengirim `nik`, `user_id`, `patient_profile_id`, atau `role` di body. Query ID juga tidak memilih pemilik lain.

Request memakai **multipart/form-data**, tepat dua part:

| Part | Bentuk | Batas |
| --- | --- | --- |
| `data` | Field teks berisi satu JSON object, bukan file | 64 KiB |
| `photo` | Satu file gambar PNG/JPEG | 8 MiB, maksimal 25 juta pixel |

Total request dibatasi **8 MiB + 128 KiB**, termasuk framing/headers/epilogue multipart. Field/part tambahan, duplikat `data`/`photo`, data JSON sebagai file, atau photo tanpa filename ditolak. Filename dan MIME kiriman klien tidak dipakai sebagai bukti jenis gambar; nama file asli tidak disimpan.

Isi JSON `data` (contoh sintetis):

```json
{
  "full_name": "Pasien Sintetis",
  "birth_place": "Kota Contoh",
  "birth_date": "2005-01-01",
  "gender": "Laki-laki",
  "blood_type": "O",
  "address": "Jalan Contoh 1",
  "rt": "001",
  "rw": "002",
  "village": "Desa Contoh",
  "district": "Kecamatan Contoh",
  "religion": "Islam",
  "marital_status": "Belum Kawin",
  "occupation": "Pelajar",
  "citizenship": "WNI"
}
```

Semua 14 field wajib berupa string UTF-8 tidak kosong/whitespace saja, maksimal 16 KiB per field, dengan batas total JSON di atas. Nilai asli tidak di-trim/reformat saat disimpan.

Validasi khusus:

- `birth_date`: tanggal valid `YYYY-MM-DD`, bukan di masa depan; tidak otomatis dihitung dari umur profil.
- `gender`: persis `Laki-laki` atau `Perempuan`.
- `blood_type`: `A`, `B`, `AB`, `O`, atau `-` jika tidak diketahui.
- `rt`/`rw`: 1–3 digit ASCII sebagai string, atau `-` bila tidak diketahui; nol di depan dipertahankan.
- `marital_status`: `Belum Kawin`, `Kawin`, `Cerai Hidup`, atau `Cerai Mati`.
- `citizenship`: `WNI` atau `WNA`.
- `religion`, nama, tempat lahir, alamat, kelurahan/desa, kecamatan, dan pekerjaan berupa teks bebas dengan batas di atas.
- Field JSON tambahan, tipe selain string, serta lebih dari satu JSON value ditolak.
- Foto diperiksa berdasarkan bytes: header/dimensi dibatasi sebelum decode penuh PNG/JPEG untuk menolak gambar tidak lengkap/rusak. Bytes file asli disimpan tanpa re-encode. Tidak ada OCR atau pemeriksaan keaslian KTP/keanggotaan resmi.

Contoh pengiriman browser (`ktpData` adalah object di atas, `photoFile` adalah File yang dipilih pasien):

```js
const form = new FormData();
form.append("data", JSON.stringify(ktpData));
form.append("photo", photoFile);

const response = await fetch(`${API_BASE_URL}/patients/me/ktp`, {
  method: "POST",
  credentials: "include",
  body: form
});
const result = await response.json();
```

**Jangan set Content-Type secara manual**; browser harus memasukkan boundary multipart. Periksa `response.ok`/status sebelum menampilkan sukses. Jangan mencetak data/foto pasien atau hasil fetch ke log.

Respons sukses — **`201 Created`**:

```json
{
  "message": "Patient KTP created",
  "ktp": {
    "id": 3,
    "patient_profile_id": 12
  },
  "encryption": {
    "active": ["aes", "des", "rc4"],
    "pending": []
  }
}
```

Setiap field, termasuk NIK dari profil dan foto, dienkripsi sebagai tiga representasi AES/DES/RC4. Semua hasil didekripsi dan dibandingkan byte-per-byte sebelum satu INSERT dengan transaksi default GORM. Kegagalan validasi/enkripsi tidak membuat record sebagian. Foto disimpan pada kolom bytea PostgreSQL, bukan filesystem; parsing multipart memakai reader terbatas di memori tanpa temporary upload plaintext di disk. Tidak ada perubahan pada record profil.

Unique `patient_profile_id` menolak pembuatan KTP kedua, termasuk request bersamaan. Endpoint tidak melakukan upsert/overwrite; update/delete belum disediakan. Respons tidak mengirim nama/NIK, ciphertext, foto, atau kunci; gunakan kedua GET untuk membaca milik pasien tersebut.

| Status error POST KTP | Kondisi | Contoh message |
| --- | --- | --- |
| `400` | Multipart/JSON/field tidak valid, part hilang/duplikat/tambahan, atau gambar tidak valid | Pesan validasi tanpa nilai identitas pasien |
| `401` | Cookie/token tidak valid atau user sudah dihapus | Pesan middleware atau `Unauthorized` |
| `403` | Role terbaru bukan pasien | `Only patients can access their own KTP` |
| `404` | Pasien belum mempunyai profil | `Patient profile not found` |
| `409` | Profil sudah mempunyai KTP | `Patient KTP already exists` |
| `413` | Foto, data JSON, atau total multipart melebihi batas | Pesan batas ukuran |
| `415` | Content-Type bukan multipart/form-data | `Content-Type must be multipart/form-data` |
| `500` | Pembacaan profil, kriptografi, atau penyimpanan gagal | `Unable to create patient KTP` |

Semua error memakai JSON `{"message":"..."}`. Tes alur upload/read memakai data sintetis dan repository in-memory; pengujian PostgreSQL nyata tetap perlu dilakukan terpisah.

## 11. File generik tiga cipher

`POST /api/files` menerima multipart field `file`, maksimal 8 MiB file / 9 MiB body, khusus pasien yang sudah mempunyai profil. Dokter memakai route upload bertarget di bagian 12. Upload baru selalu menghasilkan AES, DES, dan RC4 dari input sama. Semua ciphertext harus berhasil ditulis dan lolos verifikasi dekripsi dari disk sebelum metadata diinsert. Kegagalan memicu cleanup seluruh ciphertext operasi; kegagalan cleanup juga menghasilkan error. Tidak ada pemilihan satu algoritma saat upload.

`GET /api/files` dan `GET /api/files/:id` dibatasi role dan pasien subjek (bagian 12), bukan hanya user pengunggah. DTO metadata sekarang mencakup:

- `id`, `original_name`, `original_size`, `original_type`, `uploaded_by_user_id` (pengunggah), `patient_profile_id` (pasien subjek; null pada data lama yang belum dipetakan).
- `algorithm`: label AES default yang dipertahankan untuk kompatibilitas, bukan klaim hanya AES disimpan.
- `algorithms`: nama cipher dengan path nonkosong pada metadata, misalnya `["aes","des","rc4"]`; metadata tidak memverifikasi keberadaan file live.
- `cipher_set_version`: `1` untuk upload baru yang mewajibkan tiga cipher; `0` untuk legacy AES-only.
- `aes_encrypt_time_ns`, `aes_decrypt_time_ns`, `des_encrypt_time_ns`, `des_decrypt_time_ns`, `rc4_encrypt_time_ns`, `rc4_decrypt_time_ns`.

Download: **`POST /api/files/:id/decrypt?algorithm=aes|des|rc4`**, tanpa body, cookie login wajib. Algorithm default/kosong adalah `aes`; nilai lain `400`. Untuk versi 1, semua ciphertext wajib lolos autentikasi/dekripsi dan memiliki plaintext identik; satu hilang/rusak/tidak cocok menggagalkan seluruh download, tanpa partial plaintext atau fallback.

Sukses `200` binary bytes asli, `Content-Type: application/octet-stream`, attachment filename, `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`, `X-Decryption-Algorithm` berisi pilihan cipher, dan `X-Decryption-Time-Ns` untuk cipher pilihan. Kedua header dekripsi diekspos melalui CORS. Hanya timing decrypt cipher pilihan yang diupdate pada database, dan kegagalan update membuat request gagal sebelum body sukses dikirim.

Error: `400` input/algorithm invalid; `401` tidak terautentikasi; `403` role tidak diizinkan; `404` file tidak ada/tidak berwenang atau profil upload belum ada; `409` memilih DES/RC4 untuk legacy AES-only; `413` batas ukuran; `500` kegagalan storage/DB/integritas. Bentuk error tetap `{"message":"..."}`; internal error `Failed to process file`.

**Legacy dan batasan:** versi 0 tetap dapat dibaca dengan AES jika envelope valid; tidak otomatis di-backfill. AutoMigrate menambahkan empat kolom timing dan versi set cipher, tetapi migrasi tidak dijalankan dalam sesi ini. Filesystem dan PostgreSQL bukan satu transaksi crash-atomic; crash/cleanup failure dapat meninggalkan orphan. Metadata nama/tipe/ukuran tetap plaintext, DES/RC4 bukan cipher produksi, dan batas video masih 8 MiB.

Timing hanya mencakup fungsi envelope, bukan I/O/network/DB. Download versi 1 memverifikasi ketiganya sehingga latensi HTTP bukan waktu satu cipher; header melaporkan waktu fungsi dekripsi yang dipilih. Bukan benchmark berulang atau klaim seluruh assignment selesai. Detail konfigurasi, envelope, cleanup, pengukuran, dan contoh klien: [FILES.md](FILES.md).

## 12. Akses dokter dan pasien subjek lampiran

### 12.1. Model dan aturan akses

`files.user_id` tetap menyimpan **user pengunggah**; tidak diubah menjadi pasien pemilik. Kolom baru `files.patient_profile_id` merujuk `patient_profiles.id` sebagai **pasien subjek**. Satu profil dapat mempunyai banyak lampiran, sedangkan satu lampiran baru memiliki satu pasien subjek. Relasi FK memakai RESTRICT untuk penghapusan/perubahan ID profil.

Contoh sintetis: dokter user `50` mengunggah laporan untuk profil pasien `12`, milik user `7`. Metadata file memuat `uploaded_by_user_id: 50` dan `patient_profile_id: 12`; pasien user `7` berhak membaca walaupun bukan pengunggah.

**Patient profile ID berbeda dari user ID.** Ambil ID target dari directory dokter; jangan menganggap angka keduanya selalu sama.

| Operasi | Patient | Doctor |
| --- | --- | --- |
| POST `/api/files` | Upload untuk profil sendiri; harus sudah punya profil | `403`; gunakan upload dengan target eksplisit |
| GET `/api/files` | Lampiran profil sendiri, termasuk laporan dokter, ditambah legacy NULL milik pengunggah sendiri | Semua lampiran yang terhubung ke profil pasien dalam satu rumah sakit, ditambah legacy NULL unggahannya sendiri |
| GET `/api/files/:id` dan POST `/api/files/:id/decrypt` | Hanya lampiran profil sendiri / legacy NULL unggahan sendiri | Semua lampiran terhubung ke profil pasien / legacy NULL unggahan sendiri |
| Directory dan route `/api/doctor/...` | `403` | Diizinkan sesuai kontrak berikut |

Setiap request membaca role terbaru dari database sekali melalui helper backend; role/ownership kiriman klien tidak dipercaya. Uploader tidak otomatis berhak jika bukan lagi dokter dan file tersebut memiliki pasien subjek lain. Role tidak disimpan/diambil dari JWT sebagai sumber izin. Role di luar patient/doctor ditolak. User dihapus menghasilkan `401` meskipun JWT belum kedaluwarsa.

Metadata, daftar, download, dan update timing memakai predicate akses yang sama. Pasien lain menerima `404` untuk ID file yang tidak berwenang, bukan bocoran isi/keberadaan file. Tidak ada pembatasan penugasan dokter per pasien atau multi-hospital.

### 12.2. Directory — `GET /api/doctor/patients`

Cookie login role doctor wajib. Tanpa body, pagination, atau filter. Mengembalikan profil pasien beserta identitas akun minimum, urutan `patient_profile_id` ASC:

```json
{
  "patients": [
    {
      "patient_profile_id": 12,
      "user_id": 7,
      "name": "Pasien Sintetis",
      "username": "patient_demo"
    }
  ]
}
```

Contoh sintetis. Akun pasien yang belum membuat profil tidak muncul dan belum bisa menjadi target lampiran. Hasil kosong tetap `200` dengan `{"patients":[]}`. Tidak mengirim NIK/BPJS, password hash, key, atau ciphertext. Header `Cache-Control: no-store`.

Error: `401` cookie/user tidak valid; `403` bukan doctor; `500` database gagal, message `Failed to list patients`. Bentuk error JSON `{"message":"..."}`.

### 12.3. Dokter upload — `POST /api/doctor/patients/:patient_profile_id/files`

Body multipart `file`, batas 8 MiB file / 9 MiB body, sama dengan upload generik. Profil target wajib ada dan merupakan pasien; ID path harus integer positif dalam rentang backend. Tidak memerlukan profil pasien untuk akun dokter pengunggah.

Contoh path: `/api/doctor/patients/12/files`. Backend menetapkan pengunggah dari akun login dan pasien dari profil target; field/query `user_id`, `role`, atau `patient_profile_id` lain tidak mengubah keduanya. Pasien tidak dapat memakai route ini walaupun memilih ID dirinya sendiri.

Sukses `201` memakai bentuk upload yang sama: `{"message":"File encrypted and saved successfully","file":{...metadata...}}`. Field metadata tambahan pada semua respons file adalah:

```json
{
  "id": 42,
  "uploaded_by_user_id": 50,
  "patient_profile_id": 12
}
```

Cuplikan sintetis ini hanya memperlihatkan field identitas; nama/ukuran/algorithms/versi/timing tetap mengikuti bagian 11. Path storage dan kunci tidak dikirim. AES/DES/RC4 tetap dibuat lengkap dan diverifikasi sebelum insert; kegagalan DB/FK memicu cleanup ciphertext seperti upload biasa.

Error: `400` ID/part file invalid, `401` cookie/user invalid, `403` bukan doctor, `404` target profil tidak ditemukan, `413` batas ukuran, `500` gangguan data/storage/kriptografi. Error file internal tetap `Failed to process file`; tidak mengirim plaintext sebagian.

### 12.4. Dokter list — `GET /api/doctor/patients/:patient_profile_id/files`

Role doctor dan target profil valid wajib. Tanpa body. Respons `200` adalah `{"files":[...metadata...]}`, ID file DESC, `Cache-Control: no-store`. Hanya lampiran dengan patient_profile_id tersebut; bukan semua unggahan dokter itu, dan bukan legacy yang belum ditetapkan pasiennya. Kosong memakai array `[]`. Profil tidak ada `404`, ID invalid `400`, pasien yang memakai route `403`.

Untuk detail/download gunakan endpoint file yang sama:

```text
GET /api/files/42
POST /api/files/42/decrypt?algorithm=aes
POST /api/files/42/decrypt?algorithm=des
POST /api/files/42/decrypt?algorithm=rc4
```

Pasien subjek menggunakan endpoint yang sama dengan cookie pasiennya; tidak perlu meniru akun dokter atau mengirim ID pengunggah.

### 12.5. Perubahan klien dan migrasi

- Pasien sekarang harus membuat profil sebelum POST `/api/files`; tanpa profil mendapat `404 Patient profile not found`.
- Dokter yang sebelumnya memakai POST `/api/files` harus beralih ke route bertarget, karena upload tanpa pasien eksplisit sekarang `403`.
- AutoMigrate menambahkan nullable patient_profile_id dan FK/index melalui model yang sudah ada. `user_id` pengunggah dan ciphertext lama tidak diubah. Tidak ada SQL migration baru atau migrasi database yang dijalankan dalam sesi ini.
- Row lama bernilai NULL **tidak otomatis dipetakan**. Hanya pengunggahnya sendiri tetap boleh mengakses; dokter lain tidak memperoleh akses global ke data tanpa pasien yang diketahui. Pemetaan historis perlu diverifikasi pengelola secara terpisah, bukan ditebak dari pengunggah.
- ID relasi/pengunggah tetap metadata plaintext untuk query/FK. Enkripsi berkas tidak mengautentikasi relasi ini terhadap administrator database; jangan mengklaim semua metadata terlindungi atau audit trail lengkap.

Scope perubahan ini: directory pasien dan otorisasi lampiran. Endpoint dokter untuk detail profil lengkap/KTP, rekam medis, draf/final, penulisan catatan, dan history belum ditambahkan. Akun dokter tetap disediakan pengelola; tidak ada register publik yang bisa memilih role doctor.
