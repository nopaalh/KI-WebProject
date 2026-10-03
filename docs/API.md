# Dokumentasi API Backend — Panduan FrontEnd

Dokumen ini mengikuti implementasi backend saat ini, bukan rancangan fitur mendatang. Sumber utama: router, controller autentikasi, service autentikasi, dan generator JWT di `backend/`.

## 1. Status implementasi

| Fitur | Status |
| --- | --- |
| Register pengguna | Tersedia |
| Login dengan username dan password | Tersedia |
| Penerbitan JWT saat login | Tersedia |
| Middleware untuk verifikasi JWT / endpoint terproteksi | Belum tersedia |
| Profil pengguna / endpoint `/me` | Belum tersedia |
| Refresh token dan logout server-side | Belum tersedia |
| Upload, daftar, download, dan enkripsi/dekripsi file | Belum tersedia; baru ada model database `File` |
| CORS untuk akses lintas origin dari browser | Belum dikonfigurasi |

**FrontEnd saat ini bisa mengintegrasikan form register dan login.** Jangan menganggap model database sebagai endpoint API yang sudah dapat dipanggil.

## 2. Base URL dan konfigurasi

```text
http://localhost:<SERVER_PORT>/api
```

Port diambil dari environment variable `SERVER_PORT`. Tidak ada port default yang ditetapkan di kode. Contoh dalam dokumen ini menggunakan `SERVER_PORT=8080`, sehingga base URL-nya adalah `http://localhost:8080/api`; sesuaikan dengan konfigurasi backend yang dijalankan.

Semua endpoint yang tersedia:

| Method | Path lengkap | Autentikasi | Fungsi |
| --- | --- | --- | --- |
| `POST` | `/api/users/register` | Tidak diperlukan | Membuat pengguna |
| `POST` | `/api/users/login` | Tidak diperlukan | Mendapatkan JWT |

Tidak ada prefix versi seperti `/v1`.

### Header request

```http
Content-Type: application/json
Accept: application/json
```

Kirim body JSON, bukan `FormData` atau form URL-encoded. Kedua endpoint tidak memerlukan header `Authorization`.

### Menjalankan backend untuk integrasi lokal

Backend menggunakan Go (versi pada `backend/go.mod`: `1.26.0`) dan PostgreSQL. Buat file `backend/.env` secara lokal dengan konfigurasi berikut:

```dotenv
SERVER_PORT=8080
DATABASE_URL=postgres://app_user:local_password@localhost:5432/app_db?sslmode=disable
JWT_SECRET=ganti-dengan-secret-acak-yang-kuat
```

Nilai di atas hanya contoh; sesuaikan kredensial database. `sslmode=disable` hanya untuk contoh lingkungan lokal. Jangan commit kredensial atau secret ke repository.

Jalankan dari direktori `backend/`:

```sh
go run .
```

Backend memanggil `godotenv.Load()` dan akan berhenti jika file `.env` gagal dimuat. Saat startup, backend juga menghubungkan database dan menjalankan migrasi model `User` serta `File`.

`DATABASE_URL` dan `JWT_SECRET` hanya untuk backend. **Jangan memasukkan `JWT_SECRET` atau kredensial database ke environment FrontEnd yang dibundel ke browser.**

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
| `409 Conflict` | Username ditemukan sudah terdaftar saat pengecekan | `username already exist` |
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

Contoh tersebut bukan teks tetap untuk semua `400`. Gunakan status HTTP untuk menentukan perilaku UI; jangan parsing string validasi Gin. Kegagalan insert karena benturan username yang terjadi setelah pengecekan awal juga bisa masuk sebagai `500`, sehingga tidak semua kasus duplikat dijamin menghasilkan `409`.

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
  "message": "Login successful",
  "token": "<JWT-yang-dihasilkan-backend>"
}
```

Nilai `token` di atas adalah ilustrasi, bukan token untuk dipakai mengakses server. Response tidak menyertakan objek pengguna, `refresh_token`, `token_type`, atau `expires_in`.

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
curl -i -X POST http://localhost:8080/api/users/login \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  --data '{"username":"nopal","password":"contoh-password-kuat"}'
```

## 5. JWT dan alur autentikasi FrontEnd

Token ditandatangani dengan **HS256**, menggunakan `JWT_SECRET` milik backend, dan memiliki masa berlaku **24 jam sejak dibuat**.

Payload yang dibuat backend:

```json
{
  "user_id": 1,
  "username": "nopal",
  "exp": 1791158400
}
```

Angka di atas hanya contoh. `user_id` adalah ID database, sedangkan `exp` adalah Unix timestamp dalam **detik**, bukan milidetik.

Alur yang dapat diterapkan sekarang:

1. Register melalui `/api/users/register`.
2. Setelah `201`, tampilkan informasi sukses dan arahkan ke halaman login.
3. Login melalui `/api/users/login`.
4. Setelah `200`, simpan token dalam state autentikasi FrontEnd dan pindah ke tampilan setelah login.
5. Untuk logout lokal, hapus token dari state/penyimpanan FrontEnd.
6. Jika token sudah kedaluwarsa, minta pengguna login lagi; belum ada endpoint refresh.

**Batasan penting:** backend baru menerbitkan JWT, belum memverifikasi JWT pada request berikutnya. Belum ada endpoint terproteksi maupun kontrak header Bearer yang sudah digunakan oleh middleware. Menampilkan halaman setelah login di FrontEnd tidak berarti API lain sudah terlindungi.

Logout lokal tidak membatalkan token yang sudah diterbitkan. Tidak ada mekanisme revokasi server-side saat ini. Decode payload di browser boleh digunakan untuk tampilan atau pemeriksaan waktu kedaluwarsa, tetapi bukan bukti bahwa token sah.

Untuk contoh integrasi, gunakan penyimpanan token di memory/state. Jika ingin persistensi melalui `localStorage` atau `sessionStorage`, pertimbangkan risiko pencurian token melalui XSS. Backend saat ini tidak mengirim cookie autentikasi `HttpOnly`.

## 6. Contoh integrasi JavaScript dengan fetch

Contoh ini menggunakan path relatif `/api`. Artinya FrontEnd harus berada pada origin yang sama dengan backend atau menggunakan proxy development yang meneruskan `/api` ke backend.

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

// Contoh state sederhana; sesuaikan dengan state management aplikasi.
let authToken = null;

async function submitLogin(username, password) {
  try {
    const result = await login({ username, password });
    authToken = result.token;
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

function logoutLocal() {
  authToken = null;
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
  token: string;
};

type ErrorResponse = {
  message: string;
};
```

Tipe error tersebut berlaku untuk response JSON dari controller autentikasi, bukan jaminan untuk semua response server/proxy atau path yang tidak terdaftar.

## 7. CORS dan checklist integrasi

Router saat ini menggunakan `gin.Default()` tanpa middleware CORS. Jika FrontEnd dan backend berbeda origin, misalnya `http://localhost:5173` dan `http://localhost:8080`, request JSON dari browser memerlukan penanganan CORS/preflight yang belum tersedia.

Untuk development, gunakan salah satu pendekatan:

- Konfigurasikan proxy dev server FrontEnd agar request relatif `/api/...` diteruskan ke `http://localhost:<SERVER_PORT>/api/...`, **tanpa menghapus prefix `/api`**.
- Tambahkan konfigurasi CORS di backend sebagai pekerjaan terpisah jika ingin akses langsung lintas origin.

Jangan gunakan `mode: "no-cors"` sebagai solusi; response menjadi opaque dan JSON/token tidak dapat dibaca. Request `curl` yang berhasil belum membuktikan bahwa akses lintas origin dari browser sudah bekerja.

Checklist FrontEnd:

- [ ] Base URL atau target proxy sesuai `SERVER_PORT` backend.
- [ ] Body dikirim sebagai JSON menggunakan nama field yang benar.
- [ ] Form login memakai `username`, bukan `email`.
- [ ] Register sukses ditangani sebagai `201`, bukan hanya `200`.
- [ ] Register tidak dianggap otomatis login.
- [ ] Error ditangani berdasarkan status HTTP; pesan internal `500` sebaiknya tidak ditampilkan mentah ke pengguna.
- [ ] Error jaringan/CORS ditangani terpisah dari kegagalan kredensial.
- [ ] Token tidak dicetak ke log atau dibagikan; komunikasi produksi menggunakan HTTPS.
- [ ] UI fitur profil/file tidak memanggil endpoint yang belum tersedia.

## 8. Referensi implementasi

Path relatif terhadap root project:

- `backend/routers/router.go` — prefix `/api` dan setup router.
- `backend/routers/userRouter.go` — registrasi route register/login.
- `backend/controllers/authController.go` — request, validasi binding, status HTTP, dan response JSON.
- `backend/services/authService.go` — pengecekan username, bcrypt, dan alur login.
- `backend/repositories/userRepository.go` — pencarian username dan penyimpanan pengguna.
- `backend/utils/jwt.go` — algoritma, claims, dan masa berlaku JWT.
- `backend/main.go` dan `backend/config/database.go` — environment, startup, dan database.
- `backend/models/user.go` dan `backend/models/file.go` — model database, bukan kontrak response API.

Jika backend berubah, perbarui dokumen ini berdasarkan router dan controller yang benar-benar terpasang.
