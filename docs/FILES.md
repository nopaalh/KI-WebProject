# API File — AES, DES, dan RC4

Panduan operasional untuk `/api/files`; ringkasan kontrak ada di [API.md](API.md#11-file-generik-tiga-cipher). Upload baru selalu membuat **tiga ciphertext dari bytes input yang sama**, bukan memilih satu cipher. File tetap disimpan di disk server, berbeda dari foto KTP yang disimpan di PostgreSQL.

## 1. Konfigurasi dan persistensi

- `FILE_ENCRYPTION_KEY`: master key 32 byte, dikonfigurasi sebagai tepat 64 karakter hex. Buat sekali dengan `openssl rand -hex 32`, simpan aman, dan gunakan kembali setelah restart.
- `FILE_STORAGE_DIR`: default `storage/uploads`, relatif terhadap working directory backend; diselesaikan menjadi path absolut. Jalankan backend dari `backend/` bersama `.env` yang valid.
- Database, `JWT_SECRET`, dan `SERVER_PORT` tetap mengikuti [API.md](API.md#2-base-url-dan-konfigurasi).
- Jangan commit/mengirim key ke frontend atau menyimpannya di database. Kehilangan key membuat ciphertext tidak bisa dipulihkan; rotasi/re-enkripsi otomatis belum tersedia.

Storage dibuat dengan mode `0700`; ciphertext memakai nama acak `aes-*.bin`, `des-*.bin`, `rc4-*.bin` dengan mode `0600`. Nama upload tidak dipakai sebagai path. Database menyimpan pengunggah, pasien subjek, metadata, tiga path, versi set cipher, dan enam timing; **path dan key tidak dikirim ke klien**.

Storage tidak boleh menjadi public static hosting. Pakai volume durable; backup database, ciphertext, dan key secara konsisten dengan key disimpan terpisah. Restore/pindah storage harus mempertahankan atau memperbarui path absolut dengan benar.

### All-or-nothing dan batas kegagalan

1. Baca input dengan batas ukuran.
2. Untuk setiap AES/DES/RC4, ukur enkripsi, tulis ciphertext dengan nama acak, `Sync`, lalu tutup file.
3. Baca ulang ciphertext yang benar-benar berada di disk, dekripsi, dan bandingkan bytes dengan input awal.
4. Baru setelah ketiganya berhasil, insert satu record metadata lengkap dengan `cipher_set_version=1`.
5. Kegagalan enkripsi, write/close, verifikasi, atau insert mengembalikan error dan mencoba menghapus **semua** ciphertext dari operasi tersebut. Kegagalan cleanup juga dilaporkan sebagai error internal, tidak dianggap sukses.

**Filesystem dan PostgreSQL bukan transaksi atomik bersama.** Ini menjamin all-or-nothing pada jalur request normal, bukan crash-atomic/distributed transaction. Crash di antara penulisan dan insert dapat meninggalkan orphan; kegagalan filesystem saat rollback dapat meninggalkan artefak yang perlu dibersihkan. Tidak ada orphan reconciler otomatis atau jaminan pemulihan dari semua hasil commit database yang ambigu. Jangan mengklaim tahan semua power-loss hanya karena memakai `Sync`.

### Schema dan data lama

AutoMigrate menambahkan empat kolom timing DES/RC4 (`BIGINT NOT NULL DEFAULT 0`) dan `cipher_set_version` (`NOT NULL DEFAULT 0`) melalui model `File`. Tidak ada SQL migration baru atau migrasi database yang dijalankan dalam sesi implementasi ini. SQL `000001` yang lama hanya menangani timing AES; bukan migrasi lengkap untuk perubahan ini.

| Versi | Makna |
| --- | --- |
| `0` | Record historis AES-only. Kedua path DES/RC4 harus kosong untuk jalur kompatibilitas. |
| `1` | Upload baru; ketiga path/ciphertext wajib lengkap dan cocok. Path hilang tidak mengaktifkan fallback legacy. |

Data lama **tidak otomatis di-backfill**. AES lama yang valid tetap dapat diunduh melalui AES/default. Memilih DES/RC4 pada versi 0 menghasilkan `409`; upload ulang original untuk membuat record baru lengkap. Record metadata-only/tanpa envelope AES valid tetap tidak dapat didekripsi.

Default timing `0` pada record lama bukan hasil benchmark. Metadata database, termasuk versi/relasi, bukan bukti kebal manipulasi administrator.

## 2. Autentikasi dan ownership

Endpoint menggunakan cookie HttpOnly `access_token` melalui AuthMiddleware, bukan Bearer token. User ID berasal dari JWT; role terbaru dibaca dari database sekali per request. `files.user_id` adalah pengunggah, **bukan** pasien subjek; `files.patient_profile_id` merujuk profil pasien.

Pasien dapat membaca lampiran profil sendiri, termasuk unggahan dokter. Dokter boleh membaca semua lampiran yang terhubung ke profil pasien dalam satu rumah sakit. Legacy tanpa pasien (NULL) hanya tetap diakses pengunggah sendiri, bukan dibuka global. Predicate yang sama dipakai untuk metadata/list/download/update timing; pasien yang tidak berwenang mendapat `404`.

Pasien upload lewat POST `/api/files` setelah membuat profil. Dokter upload melalui POST `/api/doctor/patients/:patient_profile_id/files`; upload generik tanpa target ditolak `403`. Directory dan list per pasien: GET `/api/doctor/patients` serta GET `/api/doctor/patients/:patient_profile_id/files`. Kontrak lengkap, perubahan klien, dan migrasi: [API.md bagian 12](API.md#12-akses-dokter-dan-pasien-subjek-lampiran). Query/body ID klien tidak mengubah pemilik atau role.

## 3. Kontrak endpoint

### Upload — `POST /api/files`

- Khusus role patient, target profil sendiri ditentukan backend. Profil belum ada: `404`; role doctor: gunakan endpoint upload bertarget.
- `multipart/form-data`, field file `file`; jangan set Content-Type manual di browser.
- Maksimal **8 MiB** input dan **9 MiB** seluruh body termasuk multipart. Batas video belum dinaikkan; pipeline masih in-memory.
- Tidak ada pemilihan cipher saat upload: ketiganya selalu digunakan.
- `201 Created`: `{"message":"File encrypted and saved successfully","file":{...metadata...}}`.

### Metadata dan list

`GET /api/files/:id` menghasilkan `{"file":{...}}`; `GET /api/files` menghasilkan `{"files":[...]}` atau `{"files":[]}`. DTO sama untuk upload/list/detail:

```json
{
  "id": 1,
  "uploaded_by_user_id": 50,
  "patient_profile_id": 12,
  "original_name": "laporan.pdf",
  "original_size": 4096,
  "original_type": "application/pdf",
  "algorithm": "AES-128-CBC-HMAC-SHA256",
  "algorithms": ["aes", "des", "rc4"],
  "cipher_set_version": 1,
  "aes_encrypt_time_ns": 0,
  "aes_decrypt_time_ns": 0,
  "des_encrypt_time_ns": 0,
  "des_decrypt_time_ns": 0,
  "rc4_encrypt_time_ns": 0,
  "rc4_decrypt_time_ns": 0
}
```

ID/ukuran/nama adalah contoh sintetis; timing nol di contoh **hanya penanda format, bukan angka hasil**. Operasi nyata memakai durasi terukur.

- `algorithm` dipertahankan untuk kompatibilitas sebagai **label cipher download default AES**, bukan daftar seluruh cipher.
- `algorithms` berasal dari path nonkosong pada metadata; bukan pemeriksaan integritas/keberadaan file live. Legacy biasanya `["aes"]`; metadata-only dapat `[]`.
- `cipher_set_version` membedakan format legacy dan tiga cipher; ini terpisah dari apakah pasien subjek sudah ditetapkan.
- `uploaded_by_user_id` adalah identitas pengunggah; `patient_profile_id` adalah profil pasien subjek, nullable pada row lama. Keduanya tidak harus sama dan yang kedua bukan user ID.
- AutoMigrate menambahkan FK/index nullable pasien; record lama tidak otomatis dipetakan. Relasi membatasi penghapusan profil yang masih mempunyai lampiran.
- `original_name` dibersihkan menjadi basename. `original_type` adalah metadata multipart dari klien, bukan validasi format isi.
- GET metadata/list tidak membaca disk atau mengukur ulang; tetap menyertakan record historis. Tidak ada ciphertext/path/key dalam respons.

### Download — `POST /api/files/:id/decrypt?algorithm=aes|des|rc4`

Tanpa body. Parameter default/kosong adalah `aes`; nilai selain tiga nama lowercase tersebut ditolak `400`. Parameter hanya memilih representasi yang diukur/dilaporkan, bukan mengubah cara upload disimpan.

Untuk versi 1, backend membaca, mengautentikasi, dan mendekripsi **ketiga ciphertext**, membandingkan hasil byte-per-byte dan ukuran plaintext. Satu salinan hilang/rusak/tidak cocok membuat **seluruh download gagal**, termasuk ketika cipher lain dipilih. Tidak ada partial plaintext atau fallback. Symlink, path di luar storage, prefix cipher salah, dan envelope melebihi batas ditolak.

Setelah verifikasi lengkap, hanya timing dekripsi cipher yang dipilih diperbarui pada row owner-scoped. Jika update DB gagal, download gagal sebelum mengirim body sukses.

```http
HTTP/1.1 200 OK
Content-Type: application/octet-stream
Content-Disposition: attachment; filename=laporan.pdf
X-Decryption-Algorithm: des
X-Decryption-Time-Ns: 0
X-Content-Type-Options: nosniff
Cache-Control: no-store
```

Header timing nol di atas ilustrasi format, bukan hasil. Body berisi bytes asli, bukan JSON/Base64/envelope. Header algoritma/timing serta Content-Disposition diekspos oleh CORS. Semua respons controller memakai no-store; jangan mengandalkan header itu pada setiap error middleware.

| Status | Kondisi |
| --- | --- |
| `400` | Multipart/file/ID tidak valid atau algorithm bukan aes/des/rc4 |
| `401` | Cookie/token/akun autentikasi tidak valid |
| `403` | Role tidak berwenang, termasuk pasien pada route dokter atau dokter pada upload tanpa target |
| `404` | Record tidak ada/tidak berwenang menurut subject, atau profil target upload belum ada |
| `409` | Memilih DES/RC4 untuk record legacy AES-only |
| `413` | File >8 MiB atau body >9 MiB |
| `500` | Gangguan disk/DB, set ciphertext tidak lengkap/rusak/tidak cocok, key salah, atau update timing gagal |

Error selalu JSON `{"message":"..."}`; internal error memakai `Failed to process file`. Periksa status sebelum membaca sukses sebagai blob.

## 4. Timing dan batas evaluasi

Enam kolom timing mengukur fungsi envelope masing-masing cipher dengan `time.Now`/`time.Since`, dalam nanodetik. Termasuk derivasi kunci, alokasi, mode/padding/drop, nonce/IV acak saat enkripsi, dan HMAC; bukan hanya putaran cipher.

Tidak termasuk network, parsing multipart, disk read/write/Sync, DB, serialisasi, atau perbandingan bytes. Pada download versi 1, semua cipher diverifikasi tetapi **header dan satu kolom yang diperbarui hanya milik cipher pilihan**. Latensi HTTP mencakup verifikasi ketiganya; jangan menganggapnya waktu satu algoritma. Timing dua cipher lain tidak ditimpa pada request itu.

Kolom decrypt berawal dari verifikasi upload dan kemudian menyimpan sampel terakhir untuk cipher yang dipilih; bukan riwayat/rata-rata. Request bersamaan dapat saling menimpa. Enkripsi timing tidak berubah saat download. Konversi ms = ns/1e6.

Ini instrumentasi satu sampel, bukan benchmark ketat. Evaluasi wajib memakai input sama, variasi ukuran, pengulangan, catatan lingkungan, serta memisahkan biaya cipher dari I/O. Fixtures bernama PDF/DOCX/MP4 pada tes adalah bytes sintetis untuk round-trip, **bukan bukti validitas format media atau hasil benchmark**.

## 5. Envelope dan batas keamanan

| Cipher | Envelope |
| --- | --- |
| AES | `AES1 || IV 16 B || AES-128-CBC/PKCS7 || HMAC-SHA256` |
| DES | `DES1 || nonce 16 B || DES-CTR || HMAC-SHA256` |
| RC4 | `RC41 || nonce 16 B || RC4-drop1536 || HMAC-SHA256` |

HMAC diverifikasi sebelum dekripsi. Utilitas yang sudah ada memakai pemisahan kunci enkripsi/MAC melalui label derivasi per cipher; DES/RC4 memakai nonce dalam derivasi encryption key. Master key tidak dikirim ke frontend.

**DES dan RC4 tidak aman untuk produksi; implementasi buatan tangan bersifat akademik.** Menyimpan tiga salinan juga membuat kerahasiaan bergantung pada cipher terlemah. Metadata nama/tipe/ukuran file masih plaintext; integrasi ini bukan penyelesaian perlindungan semua metadata atau kepatuhan regulasi.

## 6. Contoh penggunaan

Dengan cookie jar hasil login dan contoh ID 1 (ganti dengan ID respons upload):

```sh
curl --fail-with-body -b cookies.txt http://localhost:8080/api/files -F 'file=@./laporan.pdf'
curl --fail-with-body -b cookies.txt -X POST 'http://localhost:8080/api/files/1/decrypt?algorithm=des' -o laporan-des.pdf
cmp ./laporan.pdf ./laporan-des.pdf
curl --fail-with-body -b cookies.txt -X POST 'http://localhost:8080/api/files/1/decrypt?algorithm=rc4' -o laporan-rc4.pdf
cmp ./laporan.pdf ./laporan-rc4.pdf
```

Periksa exit code/status sebelum cmp: curl --fail-with-body dapat menulis JSON error ke file output. Cookie jar dan hasil dekripsi bersifat privat; jangan commit.

```js
async function downloadFile(id, algorithm = "aes") {
  const response = await fetch(`/api/files/${id}/decrypt?algorithm=${encodeURIComponent(algorithm)}`, {
    method: "POST",
    credentials: "include"
  });
  if (!response.ok) throw new Error((await response.json()).message);
  return response.blob();
}
```

Untuk fetch lintas origin gunakan base URL backend dan credentials. CORS dev tetap hanya `http://localhost:5173`; object URL/blob yang dipakai UI perlu dibersihkan setelah dipakai. `attachment` pada fetch tidak otomatis membuka dialog download.

## 7. Verifikasi

Tes mencakup round-trip tiap cipher, file kosong/binary, kepemilikan, set hilang/rusak/tidak cocok, path/symlink, rollback kegagalan enkripsi/write/verifikasi/DB, cleanup failure, kompatibilitas legacy, serta query/timing whitelist DryRun. Tidak ada migrasi atau PostgreSQL nyata yang dijalankan dalam sesi ini. Smoke harness lama perlu menyesuaikan metadata serta membuat profil pasien sebelum upload; bukan bukti alur tiga cipher/relasi lampiran sudah diuji end-to-end, dan bukan bagian CI.
