# Backend Handoff — Portal Pasien dan Rumah Sakit

> Revisi ini menggantikan skenario handoff lama: “sistem internal untuk tenaga medis; hanya pengunggah yang dapat mendekripsi”. Skenario baru melibatkan pasien dan petugas rumah sakit dengan kewenangan berbeda.

## 1. Tujuan dan batas aplikasi

Assignment memakai skenario **portal pasien dan rumah sakit**. Pasien menyerahkan data/dokumen pribadi dan mengikuti perkembangan hasil pemeriksaan. Petugas mengelola rekam medis serta menyerahkan hasil kepada pasien terkait.

Scope hanya **satu rumah sakit**. Tidak perlu multi-hospital, microservices, penugasan dokter per pasien, message broker, blockchain, atau WebSocket.

Arsitektur yang disepakati:

- Satu frontend.
- Satu backend Go; gunakan struktur Gin/GORM yang sudah tersedia.
- PostgreSQL untuk data aplikasi.
- File terenkripsi disimpan di server.
- Backend menjadi sumber kebenaran autentikasi, otorisasi, versi, dan status rekam medis.
- Refresh atau polling API untuk pembaruan tampilan pasien.

## 2. Status aktual vs rencana

Baseline checkout yang diperiksa adalah `b00671d`. Commit ini hanya referensi pemeriksaan, bukan jaminan selalu merupakan versi remote terbaru. Periksa kode dan Git sebelum mulai bekerja.

| Bagian | Status saat handoff ini ditulis |
| --- | --- |
| Register/login, hash password bcrypt, JWT lewat cookie HttpOnly | Sudah ada dalam baseline |
| Model `User` dan `File` | Sudah ada dalam baseline |
| Upload, daftar/metadata file sendiri, dekripsi file AES | Sudah ada; query dibatasi `user_id` |
| AES-128-CBC + HMAC-SHA256, IV acak, verifikasi round-trip saat upload | Sudah ada; AES buatan tangan untuk edukasi, bukan implementasi produksi |
| Durasi enkripsi/dekripsi AES | Sudah dicatat; belum merupakan evaluasi tiga cipher yang lengkap |
| Batas upload | Saat ini 8 MiB; bukan 25 MiB |
| DES dan RC4 dalam pipeline penyimpanan | Belum tersedia pada baseline; kolom path di model bukan bukti cipher sudah berjalan |
| Role `patient`/`doctor` dan `GET /api/users/me` | Ada perubahan kode lokal dari sesi ini, belum di-commit/push; jangan dianggap sudah tersedia bagi anggota tim lain |
| Relasi pasien, rekam medis, draf/final, version history, akses dokter ke data pasien | Rencana; belum diimplementasikan dalam baseline yang diperiksa |
| Frontend final portal rumah sakit | Belum tersedia di checkout yang diperiksa |
| Perlindungan seluruh data sensitif dengan tiga cipher | Belum terpenuhi; nama user dan metadata file masih berupa field biasa |

Perubahan lokal role menyediakan fondasi identitas saja, **belum RBAC rumah sakit lengkap**. Endpoint file tetap owner-only. Kolom role didefinisikan langsung pada model `User` untuk dibuat/ditambahkan lewat `AutoMigrate` yang sudah ada; tidak memakai file SQL role terpisah. AutoMigrate belum dijalankan ke database dalam sesi ini. Detail perubahan lokal ada di [ROLES.md](ROLES.md).

Preview HTML dari ZIP, bila dipakai, adalah **demo dengan data contoh**, bukan frontend final. Login/upload/enkripsi/download dalam preview itu tidak boleh dianggap operasi backend sungguhan. Jangan menimpa repo utama dengan ZIP tanpa membandingkan kode.

## 3. Role dan hak akses yang disepakati

Dua role aplikasi: **pasien** dan **petugas rumah sakit**, termasuk dokter sebagai pengguna berwenang. Penamaan teknis `patient` dan `doctor` sedang dipakai dalam perubahan lokal; `doctor` mewakili petugas berwenang untuk scope ini, bukan rancangan hierarki profesi rumah sakit.

| Operasi | Pasien | Petugas/dokter |
| --- | --- | --- |
| Mengisi/upload data dan dokumen pribadi | Miliknya sendiri | Mengunggah hasil/laporan untuk pasien terkait |
| Melihat/mengambil data pasien dan lampirannya | Hanya miliknya sendiri | Semua pasien dalam rumah sakit ini |
| Melihat rekam medis draf/sedang diproses maupun final | Miliknya sendiri, dengan label status jelas | Semua pasien |
| Membuat/memperbarui rekam medis | Tidak | Ya, untuk pasien terkait |
| Melihat riwayat rekam medis | Miliknya sendiri | Sesuai kewenangan atas seluruh pasien |
| Mengubah/menghapus dokumen yang pasien upload sendiri | Belum disepakati; bukan kewajiban tahap ini | Detail pengelolaan belum disepakati |

Ketentuan backend:

- Register publik, jika disediakan, **hanya membuat pasien**. Mengirim `role: doctor` tidak boleh memberikan hak petugas.
- Akun petugas dibuat/disediakan pengelola; tidak perlu membangun dashboard admin hanya untuk memenuhi hal ini.
- Gunakan **role-based access control ditambah patient ownership** pada setiap endpoint terkait, termasuk detail, download, lampiran, dan history.
- `user_id` pengunggah saja tidak cukup: dokumen yang diunggah dokter untuk pasien harus dapat diambil pasien yang bersangkutan. Relasi pasien pemilik/subjek data dan identitas pengunggah/penulis perlu dibedakan.
- UI yang menyembunyikan tombol bukan mekanisme otorisasi. Backend tidak mempercayai role atau identitas pasien dari request tanpa memeriksa kewenangannya.
- Belum ada pembatasan berdasarkan dokter yang ditugaskan ke pasien; fitur tersebut sengaja di luar scope.

Interpretasi “their own data”: pasien hanya boleh mengambil data dirinya; petugas mempunyai kewenangan berdasarkan role untuk mengelola data pasien rumah sakit. Jelaskan interpretasi ini dalam laporan. Jika rubrik pengajar ternyata membatasi akses hanya pada pengunggah untuk semua role, perlu klarifikasi karena akan bertentangan dengan skenario ini.

## 4. Alur rekam medis dan riwayat

Contoh alur: pasien menjalani MRI dan menunggu hasil.

1. Petugas membuat rekam medis berstatus draf/sedang diproses.
2. Pasien dapat melihat versi sementara dengan label **hasil belum final**.
3. Dokter/petugas memperbarui catatan dan menambahkan hasil pemeriksaan/lampiran.
4. Setiap perubahan rekam medis yang disimpan menghasilkan **versi baru**, bukan menimpa riwayat sebelumnya.
5. Frontend pasien mengambil pembaruan melalui refresh atau polling API; tampilkan indikator **“Ada pembaruan”** ketika versi baru tersedia.
6. Petugas menandai hasil final setelah selesai.

Backend perlu menyimpan versi sebelumnya, identitas penulis perubahan, dan timestamp agar riwayat dapat ditelusuri. Status draf/final dan versi terbaru ditentukan backend, bukan hanya state frontend.

Detail nama status, bentuk payload, relasi tabel, dan penanganan dua petugas yang menyimpan bersamaan masih perlu disepakati. Tidak perlu email, push notification, atau WebSocket.

Batas klaim:

- Sinkronisasi memastikan tampilan mengikuti versi terbaru, bukan menjamin catatan benar secara medis.
- Audit history tidak otomatis membuktikan data kebal manipulasi oleh administrator.
- Integritas kriptografis, kebenaran medis, dan pertanggungjawaban perubahan adalah hal berbeda.

## 5. Empat jenis data dan privasi

Gunakan **data sintetis** untuk seluruh demonstrasi, fixture, dan benchmark; jangan memakai data pasien atau KTP asli.

| Jenis data assignment | Contoh dalam skenario |
| --- | --- |
| Personal data | Identitas dan kontak pasien |
| Scanned identity documents | Scan KTP/paspor sintetis |
| Office documents | Laporan pemeriksaan/tindakan dalam PDF, Word, atau format kantor lain |
| Video | Rekaman gerakan untuk evaluasi rehabilitasi atau video medis sintetis yang relevan |

Gambar MRI tidak otomatis memenuhi persyaratan video.

Informasi kesehatan merupakan data pribadi spesifik menurut UU PDP. Identitas, kontak, isi rekam medis, dan seluruh isi file pasien diperlakukan sebagai privat; jelaskan klasifikasi dengan rujukan UU PDP/GDPR yang sesuai dalam laporan. Jangan mengklaim seluruh aplikasi patuh regulasi hanya karena memakai enkripsi.

## 6. Cakupan enkripsi yang disepakati

> **Perubahan keputusan untuk profil pasien:** atas permintaan terbaru pengguna, schema `patient_profiles` menyimpan berat, tinggi, dan umur sebagai plaintext; hanya NIK dan nomor BPJS yang disiapkan sebagai tiga representasi terenkripsi. Ini pengecualian terhadap rancangan awal di bawah dan berpotensi bertentangan dengan ketentuan assignment melindungi seluruh data. Endpoint create `POST /api/patients/me/profile` sekarang menyediakan enkripsi NIK/BPJS dengan AES, DES, dan RC4 untuk record baru. Kedua kolom RC4 profil tetap nullable demi kompatibilitas record lama dari tahap AES/DES; backfill record lama belum disediakan/dijalankan. GET `/api/patients/me/profile` tersedia untuk pasien membaca profil sendiri dengan verifikasi kecocokan hasil dekripsi; GET `/api/patients/me/ktp` dan GET `/api/patients/me/ktp/photo` tersedia untuk pasien membaca data/foto sendiri; POST `/api/patients/me/ktp` tersedia untuk membuat data dan foto KTP sendiri dengan tiga cipher dalam satu record PostgreSQL. NIK bersumber dari profil; tidak ada overwrite/upsert. Update/delete belum tersedia. Kontrak endpoint KTP dan checklist lima endpoint pasien ada di API.md bagian 10. Detail schema dan batasannya ada di [PATIENT_PROFILES.md](PATIENT_PROFILES.md). Jangan menyatakan seluruh data sensitif sudah terlindungi.

Setiap isi sensitif disimpan sebagai **tiga representasi terenkripsi dari plaintext yang sama**, masing-masing AES, DES, dan RC4. Bukan pengguna memilih satu cipher saat upload lalu dua cipher lain tidak digunakan.

Cakupannya meliputi:

- Data pribadi pasien.
- Teks rekam medis, termasuk draf.
- Seluruh versi riwayat yang disimpan.
- Dokumen identitas, laporan kantor, video, dan lampiran lain.
- Metadata sensitif, misalnya nama file yang dapat mengungkap identitas atau diagnosis.

Password tetap disimpan sebagai hash password yang sesuai (bcrypt sudah digunakan), bukan plaintext atau enkripsi reversibel tiga cipher.

Metadata untuk query perlu dirancang eksplisit. Identitas relasi pasien/penulis, versi, status, waktu, ukuran, tipe file, dan lokasi ciphertext adalah **daftar yang perlu ditinjau**, bukan otomatis daftar field yang boleh plaintext. Tentukan field minimum yang dibutuhkan untuk query, risiko kebocorannya, serta field mana yang masuk payload terenkripsi. Penyimpanan `User.Name` plaintext tidak lagi dianggap pengecualian yang sudah disetujui.

Persyaratan kriptografi:

- AES dan DES memakai mode selain ECB.
- Lindungi integritas, bukan kerahasiaan saja; verifikasi autentikasi sebelum mengembalikan plaintext.
- Kunci/secret backend tidak dikirim ke frontend.
- Pengelolaan master key, pemisahan kunci per fungsi/cipher, nonce/IV, format envelope, dan kegagalan verifikasi harus dijustifikasi.
- Uji dekripsi mengembalikan bytes yang **identik** dengan input, bukan hanya file dapat dibuka.
- DES dan RC4 hanya untuk assignment/perbandingan akademik, bukan keamanan medis produksi. Menyimpan tiga representasi juga membuka jalur serangan melalui cipher terlemah.

## 7. Usulan teknis dari handoff lama — perlu konfirmasi

Bagian ini dipertahankan agar pekerjaan antar-BE tidak kehilangan konteks. **Ini usulan kontrak, bukan keputusan final seluruh tim dan bukan bukti implementasi tersedia.** Assignment membolehkan library; implementasi manual bukan kewajiban assignment berdasarkan konteks yang tersedia.

### Cipher dan penyimpanan

- AES saat ini: AES-128-CBC + PKCS7 + HMAC-SHA256, envelope `AES1`.
- Usulan DES: **single DES**, mode CTR + HMAC-SHA256, tanpa padding, kunci turunan berbeda per file; bukan 3DES. CTR dipilih agar DES bekerja sebagai pembangkit keystream yang di-XOR dengan data seperti RC4, sehingga DES dan RC4 sebanding sebagai cipher aliran dan ukuran ciphertext sama dengan plaintext.
- Usulan RC4: nonce acak per payload, kunci turunan berbeda per nonce, buang 1536 byte awal keystream, lalu HMAC-SHA256. Ini tidak menjadikan RC4 layak untuk produksi.
- Usulan master key: gunakan `FILE_ENCRYPTION_KEY` 32 byte dengan derivasi berlabel per cipher dan per fungsi enkripsi/MAC, mengikuti pola AES yang ada.
- Usulan data terstruktur: satu payload JSON sensitif per record/versi, dienkripsi tiga kali. Konsekuensinya field di dalam payload tidak dapat di-query langsung.
- Pipeline saat ini in-memory. Batas video perlu diuji berdasarkan memori dan waktu proses; angka 25 MiB dari handoff lama **hanya usulan**, bukan batas aktif.

### Usulan kontrak utilitas

```go
func EncryptDESFile(plaintext []byte, key [32]byte) ([]byte, error)
func DecryptDESFile(encrypted []byte, key [32]byte) ([]byte, error)
func EncryptRC4File(plaintext []byte, key [32]byte) ([]byte, error)
func DecryptRC4File(encrypted []byte, key [32]byte) ([]byte, error)
```

Sama seperti AES: key adalah master key, HMAC diverifikasi sebelum dekripsi, dan error tidak mengembalikan plaintext parsial.

| Envelope usulan | Format | Turunan kunci usulan |
| --- | --- | --- |
| `DES1` | `"DES1" ‖ nonce 16 B ‖ DES-CTR ‖ HMAC-SHA256` | enc = `HMAC(master, "DES1/encryption" ‖ nonce)[:8]`, mac = `HMAC(master, "DES1/authentication")`; counter 64 bit big-endian mulai dari 3377 (konstanta `desCTRStart`, tidak rahasia) |
| `RC41` | `"RC41" ‖ nonce 16 B ‖ RC4-drop1536 ‖ HMAC-SHA256` | enc = `HMAC(master, "RC41/encryption" ‖ nonce)[:16]`, mac = `HMAC(master, "RC41/authentication")` |

HMAC mencakup `magic ‖ IV/nonce ‖ ciphertext`. Label derivasi dan format ini harus disepakati sebelum producer/consumer diintegrasikan.

Pemilihan algoritma saat dekripsi, misalnya `?algorithm=aes|des|rc4`, masih usulan API. Jika dipakai, hanya memilih representasi yang dibaca; penyimpanan tetap harus menghasilkan ketiganya. CTR sekarang menjadi mode DES (bukan lagi eksperimen tambahan). Karena AES tetap CBC, selisih AES dan DES pada benchmark mencampur efek cipher dan efek mode, jadi benchmark mikro juga mengukur AES dan DES pada CBC dan CTR yang sama agar kedua efek itu terpisah. Kunci DES unik per file, sehingga nonce tidak perlu berbagi blok dengan counter dan keystream tidak pernah dipakai ulang antar file.

## 8. Usulan pembagian dan urutan kerja

Pembagian ini memakai PIC handoff lama, **perlu dikonfirmasi ulang** karena scope aplikasi berubah. Tidak menetapkan deadline atau menganggap perubahan sudah dikerjakan.

| Bagian | Usulan PIC | Cakupan |
| --- | --- | --- |
| DES/RC4 dan envelope autentikasinya | Kamal (BE-2) | Core/library sesuai keputusan tim, mode DES, derivasi kunci, format envelope, test vector dan tampering test |
| Evaluasi cipher | Kamal (BE-2), koordinasi dengan BE-1 | Benchmark mikro, pengukuran download berulang, round-trip, overhead, interpretasi |
| Role dan akses pasien/dokter | nopaalh (BE-1) | Provisioning dokter, backend role checks, patient ownership; review perubahan role lokal lebih dahulu |
| Data pasien dan rekam medis | nopaalh (BE-1) | Relasi pasien, payload sensitif, draf/final, versi, penulis, timestamp, API history |
| Integrasi tiga cipher | nopaalh (BE-1), memakai kontrak BE-2 | Penyimpanan file dan payload sensitif ×3, metadata privat, respons API, penanganan kegagalan |
| Kontrak frontend/backend | Tim frontend bersama BE-1 | Request/response, status dan error, polling/versi, download, loading/success/error state |

Urutan yang disarankan:

1. Review handoff ini: role, ownership, relasi pasien, status/versi, metadata privat, dan kontrak cipher.
2. Sepakati model/API minimum; jangan mengubah kontrak paralel tanpa koordinasi.
3. Pekerjaan cipher dapat berjalan paralel dengan role, relasi pasien, dan rekam medis.
4. Integrasikan tiga cipher untuk seluruh payload sensitif dan lampiran, termasuk history.
5. Uji alur pasien/petugas, penolakan akses tidak sah, integritas, persistensi, dan sinkronisasi.
6. Jalankan evaluasi pada implementasi yang benar-benar dipakai, kemudian tulis interpretasi laporan.

Batas upload video ditentukan dari pengujian throughput **dan** penggunaan memori, bukan throughput saja. Perubahan schema perlu mempertimbangkan record/file AES lama: jangan menganggap data lama otomatis sudah punya tiga representasi.

## 9. Evaluasi dan verifikasi

Evaluasi wajib membandingkan ketiga cipher memakai input yang sama, variasi tipe/ukuran (termasuk dokumen kecil dan video lebih besar), dan pengulangan yang memadai.

Metrik minimum yang diusulkan:

- Waktu enkripsi dan dekripsi.
- Throughput dengan satuan jelas.
- Ukuran hasil dan overhead penyimpanan.
- Keberhasilan round-trip identik secara byte.

Pisahkan waktu kriptografi dari jaringan, upload, disk, dan database. Catat lingkungan, implementasi/library, ukuran input, jumlah pengulangan, dan metode agregasi. Jangan membuat angka hasil.

Usulan pelaksanaan dari handoff lama:

- Mikro: benchmark Go berulang, misalnya `go test -bench ... -count=10`, baseline stdlib jika memakai cipher buatan tangan.
- Makro: download/dekripsi berulang, misalnya 30 kali per file per cipher; bedakan latensi endpoint dan durasi kriptografi backend.
- Ciphertext: bandingkan overhead dan variasi hasil untuk plaintext sama. Statistik byte/entropi saja bukan bukti keamanan.

Handoff lama menyebut instruksi slide “download the data multiple times” dan analisis ciphertext. Periksa materi assignment asli sebelum mengutipnya sebagai ketentuan tambahan; slide tersebut belum diverifikasi dalam revisi ini.

Uji fungsional/keamanan yang perlu tersedia:

- Register publik tidak bisa memberi role dokter.
- Pasien A tidak bisa mengambil data/lampiran/history pasien B, termasuk lewat ID yang ditebak.
- Pasien tidak bisa membuat/mengubah rekam medis rumah sakit.
- Petugas dapat mengelola data pasien sesuai scope satu rumah sakit.
- Pasien melihat draf/final dan perubahan versi miliknya tanpa kehilangan history.
- Semua jenis data kembali utuh melalui masing-masing cipher; ciphertext rusak/kunci salah ditolak tanpa plaintext parsial.
- Data yang berhasil disimpan tetap dapat didekripsi setelah restart dengan konfigurasi kunci yang benar.

## 10. Hal yang masih perlu diputuskan tim

- Konfirmasi penamaan role teknis `doctor` sebagai petugas berwenang untuk versi ini.
- Bentuk tabel/payload dan endpoint pasien, rekam medis, versi, history, serta lampiran.
- Nama status, penandaan final, dan penanganan update bersamaan.
- Metadata minimum untuk query dan perlindungan identitas pada alur autentikasi.
- Mode/format DES dan RC4, implementasi manual vs library, serta pengelolaan kunci final.
- Penanganan data lama yang baru dilindungi AES; jangan menghapus data tanpa persetujuan.
- Batas upload yang layak untuk video setelah pengujian.
- Izin edit/delete dokumen unggahan pasien, hanya jika memang diperlukan.

Search/filter, statistik dashboard, preview media, delete, dan halaman benchmark khusus bukan kewajiban eksplisit. Jangan menambahkannya sebelum kebutuhan inti terpenuhi atau ada permintaan.

## 11. Laporan dan aturan kerja

- Report maksimal **dua halaman A4**: skenario/manfaat, analisis dan alasannya, serta justifikasi arsitektur, mode, pengelolaan kunci, dan penyimpanan.
- Jelaskan batas keamanan DES/RC4, implementasi edukasi, kebocoran metadata yang tersisa, serta batas audit history. Jangan menyamakan integritas dengan kebenaran medis.
- Submission menyertakan link dan screenshot percakapan AI; hasil AI harus diverifikasi, bukan dianggap benar otomatis.
- Baca kode aktual sebelum perubahan; dokumentasi bisa tertinggal.
- Periksa status Git dan update repo bersama dengan menjaga perubahan lokal. Jangan menimpa repo dengan ZIP.
- Commit berkala, kecil, dan bermakna sesuai koordinasi tim. **Jangan commit/push dari sesi bantuan tanpa permintaan eksplisit pengguna.**
- Jika diminta push, gunakan repo bersama `https://github.com/nopaalh/KI-WebProject`, bukan fork pribadi; branch bersama `main`.
- Perubahan kontrak API wajib memperbarui `docs/API.md`/`docs/FILES.md` dan memberi tahu tim frontend.
- Cipher buatan tangan harus diverifikasi dengan test vector tepercaya dan pembanding library yang sesuai; seluruh jalur perlu diuji round-trip dan penolakan manipulasi.
