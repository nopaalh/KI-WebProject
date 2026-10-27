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
| Refresh token dan logout server-side | Belum tersedia |
| Upload, metadata per ID, dan download hasil dekripsi file AES | Tersedia; file terenkripsi di disk dan timing di database |
| Daftar seluruh file milik pengguna login | Tersedia; `GET /api/files`, tanpa pagination atau filter |
| Create profil pasien | Tersedia; `POST /api/patients/me/profile`, NIK/BPJS record baru terenkripsi AES + DES + RC4; lihat [PATIENT_PROFILES.md](PATIENT_PROFILES.md) |
| RC4 / integrasi DES | DES dan RC4 tersedia untuk create profil; pipeline file masih AES |
| CORS untuk akses lintas origin dari browser | Tersedia |

**FrontEnd dapat mengintegrasikan register, login, upload file, daftar file milik pengguna login, metadata per ID, dan download hasil dekripsi AES.** Gunakan hanya endpoint yang tercantum; belum ada endpoint RC4 atau DES. Role `patient`/`doctor`, kontrak `/me`, migrasi, dan penyediaan dokter dijelaskan di [Panduan Role](ROLES.md).

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
| `POST` | `/api/files` | Cookie `access_token` | Upload multipart, enkripc.PatientProfileCosi, dan simpan file serta timing |
| `GET` | `/api/files` | Cookie `access_token` | Seluruh metadata file milik pengguna login, urutan ID DESC, tanpa pagination/filter |
| `GET` | `/api/files/:id` | Cookie `access_token` | Metadata file milik pengguna |
| `POST` | `/api/files/:id/decrypt` | Cookie `access_token` | Dekripsi dan download byte asli; perbarui timing dekripsi |

Tidak ada prefix versi seperti `/v1`.

`GET /api/files` mengembalikan `200 OK` dengan `{"files":[...]}` (kosong: `{"files":[]}`) dan `Cache-Control: no-store`. Setiap item memakai DTO metadata yang sama dengan `GET /api/files/:id`: `id`, `original_name`, `original_size`, `original_type`, `algorithm`, `aes_encrypt_time_ns`, dan `aes_decrypt_time_ns`. Urutan **ID DESC** berarti record terbaru diinsert terlebih dahulu. Owner berasal dari `AuthMiddleware` yang sudah ada melalui cookie login; query `user_id` dari klien diabaikan dan tidak memberi akses ke file pengguna lain. Endpoint tidak menerima pagination/filter, tidak membaca disk atau mengukur ulang timing, dan tetap menyertakan record historis metadata-only. Response tidak berisi path disk atau key. Error autentikasi `401` mengikuti middleware yang sama (lihat bagian 5); kegagalan pemrosesan menghasilkan `500` dengan `{"message":"Failed to process file"}`. Contoh fetch dan curl cookie jar ada di [Panduan API File](FILES.md).

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
5. Kirim cookie pada request file dengan `credentials: "include"`; `AuthMiddleware` memvalidasi JWT dan meneruskan ID pengguna. Daftar file, metadata, dan dekripsi hanya mengakses file milik pengguna tersebut.
6. Jika request terproteksi menghasilkan `401`, minta pengguna login lagi; belum ada endpoint refresh.

Middleware membaca **cookie `access_token`**, bukan header `Authorization: Bearer ...`. Cookie hilang menghasilkan `401` dengan `message: "Missing authentication cookie"`; JWT invalid/kedaluwarsa menghasilkan `"Invalid or expired token"`. Claim ID tidak valid dapat menghasilkan `"Unauthorized: user_id missing"`; controller file juga dapat mengembalikan `"Unauthorized"`.

Belum ada endpoint logout, penghapusan cookie, atau revokasi token server-side. Menghapus state UI **tidak** menghapus cookie `HttpOnly` dan **tidak** mengakhiri autentikasi backend; JavaScript tidak dapat menghapus cookie tersebut. Jangan menyebut reset UI sebagai logout yang aman.

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

Router memakai `gin.Default()` dengan middleware CORS. Origin yang diizinkan saat ini hanya **`http://localhost:5173`**, dengan `AllowCredentials: true`. Method yang diizinkan: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS`; ini bukan berarti semuanya memiliki endpoint. Header request yang diizinkan: `Origin`, `Content-Type`, `Authorization`. Header response yang diekspos ke JavaScript: `Content-Length`, `Content-Disposition`, `X-Decryption-Time-Ns`.

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
