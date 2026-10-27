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

Status diperbarui terhadap `main` pada `57d06b2` (2026-10-27) berdasarkan kode di repo, bukan rencana. Periksa kode dan Git sebelum mulai bekerja karena dokumen bisa tertinggal.

| Bagian | Status |
| --- | --- |
| Register/login, hash password bcrypt, JWT lewat cookie HttpOnly | Sudah ada dalam baseline |
| Model `User` dan `File` | Sudah ada dalam baseline |
| Upload, daftar/metadata file sendiri, dekripsi file AES | Sudah ada; query dibatasi `user_id` |
| AES-128-CBC + HMAC-SHA256, IV acak, verifikasi round-trip saat upload | Sudah ada; AES buatan tangan untuk edukasi, bukan implementasi produksi |
| Durasi enkripsi/dekripsi AES | Sudah dicatat; belum merupakan evaluasi tiga cipher yang lengkap |
| Batas upload | Saat ini 8 MiB; bukan 25 MiB |
| DES dan RC4 dalam pipeline penyimpanan | Upload file baru menyimpan ketiga ciphertext dan memverifikasi round-trip dari disk sebelum insert; download hanya membaca dan mendekripsi cipher yang dipilih lewat `?algorithm=aes|des|rc4` (HMAC dan ukuran plaintext diperiksa), sehingga waktu tiap cipher dapat dibandingkan. Data lama AES-only tidak otomatis di-backfill; lihat API.md bagian 11 dan FILES.md |
| Role `patient`/`doctor` dan `GET /api/users/me` | Ada perubahan kode lokal dari sesi ini, belum di-commit/push; jangan dianggap sudah tersedia bagi anggota tim lain |
| Relasi pasien, rekam medis, draf/final, version history, akses dokter ke data pasien | Rencana; belum diimplementasikan dalam baseline yang diperiksa |
| Frontend final portal rumah sakit | Belum tersedia di checkout yang diperiksa |
| Perlindungan seluruh data sensitif dengan tiga cipher | Belum terpenuhi; nama user dan metadata file masih berupa field biasa |

Perubahan lokal role menyediakan fondasi identitas saja, **belum RBAC rumah sakit lengkap**. Endpoint file tetap owner-only. Kolom role didefinisikan langsung pada model `User` untuk dibuat/ditambahkan lewat `AutoMigrate` yang sudah ada; tidak memakai file SQL role terpisah. AutoMigrate belum dijalankan ke database dalam sesi ini. Detail perubahan lokal ada di [ROLES.md](ROLES.md).
| Register/login, hash password bcrypt, JWT lewat cookie HttpOnly | Sudah ada |
| Role `patient`/`doctor` (kolom `role` pada `User`) dan `GET /api/users/me` | Sudah di `main` (PR #3 dan #4); belum ada endpoint yang melayani petugas |
| Upload, daftar/metadata, dekripsi file umum (dokumen, video) | Ada, **hanya AES** (`AES1`), batas 8 MiB; belum DES/RC4; nama file masih plaintext |
| Library DES (mode CTR), RC4, CTR generik, envelope `DES1` dan `RC41` | Selesai di `backend/utils/`; diuji terhadap vektor resmi dan stdlib (`crypto/des`, `crypto/rc4`, `cipher.NewCTR`); CI hijau |
| Benchmark mikro tiga cipher | Selesai; hasil dan temuan di [benchmark.txt](benchmark.txt) |
| Profil pasien (`POST/GET /api/patients/me/profile`) | Ada; NIK dan nomor BPJS dienkripsi ×3, kolom lain plaintext (lihat bagian 6) |
| KTP pasien (`POST/GET /api/patients/me/ktp`, `GET /api/patients/me/ktp/photo`) | Ada; seluruh field dan foto dienkripsi ×3, dibaca dengan pencocokan hasil dekripsi ketiga cipher |
| Rekam medis, draf/final, version history | Belum ada model maupun endpoint |
| Frontend (Svelte) | Login, registrasi, sesi dan role; fitur lain belum diverifikasi dalam pembaruan ini |
| CI | `go test -race ./...` dan cek frontend pada PR ([CI.md](CI.md)) |
| Pengukuran makro dan analisis ciphertext | Selesai: [benchmark.txt](benchmark.txt) (mikro), [ciphertext-analysis.txt](ciphertext-analysis.txt), dan [macro-benchmark.txt](macro-benchmark.txt) (HTTP, per cipher). Pembacaan profil/KTP tetap mendekripsi ketiga cipher sekaligus; download file mendekripsi cipher pilihan saja |
| Perlindungan seluruh data sensitif dengan tiga cipher | **Belum terpenuhi:** file umum masih AES, `User.Name` dan nama file masih plaintext, serta pengecualian profil di bagian 6 |

Role baru menyediakan fondasi identitas, **belum RBAC rumah sakit lengkap**: semua endpoint pasien dan file masih berbasis pemilik. Kolom role dibuat lewat `AutoMigrate` pada model `User` dan juga tersedia sebagai migration SQL. Empat migration (`000001` timing AES, `000002` role, `000003` profil pasien, `000004` KTP) ada di `backend/migrations/`; urutan terapkan dan rollback ada di README folder itu.

Preview HTML dari ZIP, bila dipakai, adalah **demo dengan data contoh**, bukan frontend final. Login/upload/enkripsi/download dalam preview itu tidak boleh dianggap operasi backend sungguhan. Jangan menimpa repo utama dengan ZIP tanpa membandingkan kode.

## 3. Role dan hak akses yang disepakati

Dua role aplikasi: **pasien** dan **petugas rumah sakit**, termasuk dokter sebagai pengguna berwenang. Penamaan teknis `patient` dan `doctor` sudah dipakai di `main`; `doctor` mewakili petugas berwenang untuk scope ini, bukan rancangan hierarki profesi rumah sakit.

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
- `user_id` pengunggah saja tidak cukup: dokumen yang diunggah dokter untuk pasien harus dapat diambil pasien yang bersangkutan. Implementasi lampiran sekarang mempertahankan files.user_id sebagai pengunggah dan memakai files.patient_profile_id sebagai pasien subjek. Directory dokter, upload/list bertarget, serta akses file berdasarkan role/subject tersedia; kontrak dan aturan legacy ada di API.md bagian 12. Ini belum implementasi rekam medis atau akses dokter ke detail profil/KTP.
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

## 7. Cipher dan envelope — sudah diimplementasikan

Bagian ini mencatat keputusan kriptografi yang sudah dikerjakan di `backend/utils/` (`des.go`, `rc4.go`, `modes.go`, `des_file.go`, `rc4_file.go`) dan dipakai oleh endpoint profil serta KTP pasien. Cipher dibuat tangan untuk tujuan edukasi; assignment membolehkan library, tetapi implementasi manual diverifikasi terhadap `crypto/aes`, `crypto/des`, `crypto/rc4` dan test vector resmi.

### Cipher dan penyimpanan

- AES: AES-128-CBC + PKCS7 + HMAC-SHA256, envelope `AES1`.
- DES: **single DES**, mode CTR + HMAC-SHA256, tanpa padding, kunci turunan berbeda per file; bukan 3DES. CTR dipilih agar DES bekerja sebagai pembangkit keystream yang di-XOR dengan data seperti RC4, sehingga DES dan RC4 sebanding sebagai cipher aliran dan ukuran ciphertext sama dengan plaintext.
- RC4: nonce acak per payload, kunci turunan berbeda per nonce, buang 1536 byte awal keystream (RFC 4345), lalu HMAC-SHA256. Ini tidak menjadikan RC4 layak untuk produksi.
- Master key: `FILE_ENCRYPTION_KEY` 32 byte dengan derivasi berlabel per cipher dan per fungsi enkripsi/MAC, mengikuti pola AES.
- Usulan data terstruktur: satu payload JSON sensitif per record/versi, dienkripsi tiga kali. Konsekuensinya field di dalam payload tidak dapat di-query langsung.
- Pipeline saat ini in-memory. Batas video perlu diuji berdasarkan memori dan waktu proses; angka 25 MiB dari handoff lama **hanya usulan**, bukan batas aktif.

### Kontrak utilitas

```go
func EncryptDESFile(plaintext []byte, key [32]byte) ([]byte, error)
func DecryptDESFile(encrypted []byte, key [32]byte) ([]byte, error)
func EncryptRC4File(plaintext []byte, key [32]byte) ([]byte, error)
func DecryptRC4File(encrypted []byte, key [32]byte) ([]byte, error)
```

Sama seperti AES: key adalah master key, HMAC diverifikasi sebelum dekripsi, dan error tidak mengembalikan plaintext parsial.

| Envelope | Format | Turunan kunci |
| --- | --- | --- |
| `DES1` | `"DES1" ‖ nonce 16 B ‖ DES-CTR ‖ HMAC-SHA256` | enc = `HMAC(master, "DES1/encryption" ‖ nonce)[:8]`, mac = `HMAC(master, "DES1/authentication")`; counter 64 bit big-endian mulai dari 3377 (konstanta `desCTRStart`, tidak rahasia) |
| `RC41` | `"RC41" ‖ nonce 16 B ‖ RC4-drop1536 ‖ HMAC-SHA256` | enc = `HMAC(master, "RC41/encryption" ‖ nonce)[:16]`, mac = `HMAC(master, "RC41/authentication")` |

HMAC mencakup `magic ‖ IV/nonce ‖ ciphertext`. Label derivasi dan format ini sudah dipakai oleh endpoint profil dan KTP; mengubahnya membuat data yang sudah tersimpan tidak terbaca.

Pemilihan algoritma saat dekripsi, misalnya `?algorithm=aes|des|rc4`, masih usulan API dan belum ada. Jika dipakai, hanya memilih representasi yang dibaca; penyimpanan tetap harus menghasilkan ketiganya. CTR sekarang menjadi mode DES (bukan lagi eksperimen tambahan). Karena AES tetap CBC, selisih AES dan DES pada benchmark mencampur efek cipher dan efek mode, jadi benchmark mikro juga mengukur AES dan DES pada CBC dan CTR yang sama agar kedua efek itu terpisah. Kunci DES unik per file, sehingga nonce tidak perlu berbagi blok dengan counter dan keystream tidak pernah dipakai ulang antar file.

## 8. Usulan pembagian dan urutan kerja

PIC memakai pembagian handoff lama dan **perlu dikonfirmasi ulang** karena scope aplikasi berubah; baris tanpa PIC tetap usulan. Kolom Status diperbarui terhadap `main` pada `57d06b2`. Tidak menetapkan deadline.

| Bagian | PIC | Status | Cakupan |
| --- | --- | --- | --- |
| DES/RC4 dan envelope autentikasinya | Kamal (BE-2) | **Selesai** | Core, mode DES, derivasi kunci, format envelope, test vector, tampering test |
| Evaluasi cipher | Kamal (BE-2), koordinasi dengan BE-1 | Sebagian | Benchmark mikro **selesai**; analisis ciphertext (overhead, entropi), pengukuran makro (download berulang), dan interpretasi untuk laporan **belum**. Makro menunggu pembacaan per cipher |
| Role dan akses pasien/dokter | nopaalh (BE-1) | Sebagian | Role dan `GET /api/users/me` ada; provisioning dokter dan endpoint petugas belum |
| Data pasien dan rekam medis | nopaalh (BE-1) | Sebagian | Profil dan KTP (buat dan baca) ada; relasi rekam medis, draf/final, versi, penulis, history belum |
| Integrasi tiga cipher | nopaalh (BE-1), memakai kontrak BE-2 | Sebagian | Profil dan KTP sudah ×3; file umum (dokumen, video) masih AES, metadata privat belum |
| Kontrak frontend/backend | Tim frontend bersama BE-1 | Sebagian | Login, registrasi, sesi ada; form data pasien, upload dan unduh dokumen belum diverifikasi |
| Dokumentasi format `DES1` dan `RC41` di `docs/` | Kamal (usulan, belum disepakati) | Belum | `FILES.md` masih menyebut belum ada RC4/DES |
| Klasifikasi data pribadi UU PDP/GDPR untuk laporan | Tim (belum ada PIC) | Belum | Tabel data umum vs spesifik dan rujukan pasal |
| Target waktu upload dan batas ukuran video | Tim (belum ada PIC) | Belum | Dasar angka ada di [benchmark.txt](benchmark.txt) |
| Laporan 2 halaman, tautan dan screenshot percakapan AI | Semua anggota | Belum | Setiap anggota yang memakai AI menyertakan tautan dan screenshot sendiri |

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

Instruksi di slide kuliah (`04 - AES Decryption.pptx`, slide 18-19) sudah diverifikasi dan memuat ketentuan tambahan: *"Analyse the difference (i.e., running time, the resulting cipher text) between each cipher"*, *"Download the data multiple times and measure the running time!"*, *"Write a short report of the analysis result"*, dan *"The program must be bug-free to get a full mark"*. Karena itu analisis ciphertext dan pengukuran unduhan berulang termasuk kewajiban evaluasi, bukan hanya usulan.

Hasil benchmark mikro (median 10 pengulangan, satu mesin) ada di [benchmark.txt](benchmark.txt). Temuan yang memengaruhi desain: kecepatan antar-cipher bergantung implementasi (RC4 buatan tangan sekitar 150× lebih cepat dari AES dan 200× dari DES, sedangkan di stdlib urutannya berubah); dekripsi AES 2,4× lebih lambat dari enkripsinya; DES-CTR dan RC4 simetris; dan waktu upload tiga cipher sekitar 1,0 sampai 2,7 detik per MiB tergantung berapa cipher yang diverifikasi round-trip.

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
- Pengelolaan master key final (penyimpanan, rotasi). Mode, format envelope, dan implementasi manual DES/RC4 sudah diputuskan (bagian 7).
- Penanganan data lama yang baru dilindungi AES, termasuk kolom RC4 profil yang nullable; jangan menghapus data tanpa persetujuan.
- Batas upload video: tim perlu menetapkan **target waktu upload** dan apakah ketiga cipher diverifikasi round-trip saat upload. Estimasi 10 detik adalah sekitar 3,7 MiB (verifikasi ketiganya) sampai 9,6 MiB (tanpa verifikasi); angka lengkap di [benchmark.txt](benchmark.txt). Batas 8 MiB saat ini berarti sekitar 22 detik jika ketiganya diverifikasi.
- Apakah pembacaan perlu mendekripsi ketiga cipher sekaligus. Saat ini ya (pencocokan integritas), sehingga satu foto KTP menanggung sekitar 1,7 detik per MiB.
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

## 12. Permintaan Kamal (BE-2) untuk nopaalh (BE-1)

> **Status 2026-10-27 malam:** permintaan 1-3 sudah dikerjakan (upload tiga cipher, `?algorithm=`, field timing DES/RC4; migration `000005`-`000007`). Download kini hanya mendekripsi cipher pilihan (PR #27), dan pengukuran makro per cipher sudah ada di [macro-benchmark.txt](macro-benchmark.txt). Yang tersisa hanya keputusan target waktu upload dan batas video (permintaan 4). Teks di bawah dipertahankan sebagai catatan permintaan awal.

Dibuat 2026-10-27 setelah `main` di `57d06b2`. Tujuannya satu: evaluasi tiga cipher yang diminta soal (waktu dan ciphertext, unduh berulang) baru bisa lengkap kalau file umum juga disimpan dan dibaca per cipher. Semua poin di bawah mengikuti keputusan yang sudah ada; tidak ada perubahan format envelope.

### Apa yang sudah dan belum bisa diukur

| Pengukuran | Bisa sekarang? | Catatan |
| --- | --- | --- |
| Benchmark mikro cipher (laju, alokasi, CBC vs CTR) | Sudah selesai | [benchmark.txt](benchmark.txt) |
| Analisis ciphertext (overhead, entropi, plaintext sama menghasilkan ciphertext berbeda, tampering) | **Ya**, di level `utils` dengan file nyata | Dikerjakan Kamal; tidak butuh API |
| Unduh berulang (30 kali per cipher) di level kriptografi | **Ya**, di level `utils` | Dikerjakan Kamal; mengukur durasi dekripsi, bukan latensi HTTP |
| Unduh berulang di level endpoint **per cipher** | **Belum** | Terblokir permintaan 1 dan 2 |
| Waktu enkripsi/dekripsi DES dan RC4 per file di database | **Belum** | Terblokir permintaan 3 |

### Permintaan, berurutan

1. **Upload file umum menyimpan tiga envelope.** `FileService.Upload` saat ini hanya memanggil `EncryptAESFile` (batas 8 MiB). Tambahkan `EncryptDESFile` dan `EncryptRC4File` pada plaintext yang sama, tulis ke disk seperti AES, isi `DESPath` dan `RC4Path` (kolom sudah ada di model `File`), dan jaga kegagalan salah satu cipher membatalkan seluruh upload tanpa meninggalkan file yatim.
2. **Pilihan cipher saat dekripsi.** Usulan: `POST /api/files/:id/decrypt?algorithm=aes|des|rc4`, default `aes` agar klien lama tidak berubah. Hanya envelope yang dipilih yang dibaca dan didekripsi; field `algorithm` pada response mengikuti pilihan (sekarang label tetap `AES-128-CBC-HMAC-SHA256`). Nilai selain tiga itu ditolak `400`. Akses tetap owner-only. Ini yang membuat unduh berulang per cipher bisa diukur di endpoint.
3. **Kolom timing DES dan RC4.** Kamal menulis migration `000005` untuk `des_encrypt_time_ns`, `des_decrypt_time_ns`, `rc4_encrypt_time_ns`, `rc4_decrypt_time_ns` (semua `BIGINT NOT NULL DEFAULT 0`, pola sama dengan `000001`). `models/file.go` milik BE-1, jadi mohon konfirmasi nama field GORM (usulan: `DESEncryptTimeNS`, `DESDecryptTimeNS`, `RC4EncryptTimeNS`, `RC4DecryptTimeNS`) dan isi nilainya di `Upload` seperti timing AES.
4. **Putuskan target waktu upload dan batas video.** Dengan 8 MiB dan verifikasi round-trip ketiga cipher, upload sekitar 22 detik; tanpa verifikasi round-trip penuh sekitar 8 detik (angka di [benchmark.txt](benchmark.txt)). Pilihan: pertahankan 8 MiB, turunkan batas, atau verifikasi hanya sebagian cipher. Mohon putuskan bersama tim agar laporan memakai angka yang sama.
5. **Jangan ubah format envelope atau label derivasi kunci** (`DES1`, `RC41`, bagian 7). Data profil dan KTP yang sudah tersimpan jadi tidak terbaca.

### Sudah dikerjakan Kamal, relevan untuk BE-1

- `EncryptDESFile`/`DecryptDESFile`/`EncryptRC4File`/`DecryptRC4File` memakai kontrak yang sama dengan AES (`key [32]byte`, HMAC diverifikasi sebelum dekripsi).
- Migration `000001` sampai `000004` sudah dicocokkan dengan schema yang dihasilkan `AutoMigrate` dari model dan diuji naik, ulang, dan turun di PostgreSQL 16. Kamal menerapkannya ke database Supabase yang ia pakai pada 2026-10-27. Bila database kalian berbeda, ikuti urutan di `backend/migrations/README.md`. Setiap perubahan model yang menambah kolom perlu migration baru.
- Kamal akan menjalankan pengukuran makro dan analisis ciphertext setelah permintaan 1 sampai 3 masuk ke `main`. Mohon beri kabar di PR atau grup ketika sudah.
