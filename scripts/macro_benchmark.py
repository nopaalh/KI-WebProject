"""Pengukuran makro lewat HTTP terhadap backend lokal: upload/unduh file dan baca KTP (dekripsi tiga cipher).

Jalankan backend terhadap database LOKAL TERISOLASI (jangan database bersama), lalu dari folder mana pun:
    python scripts/macro_benchmark.py <port> [pengulangan]
Skrip mendaftarkan user acak sendiri dan hanya memakai library standar Python. Hasil yang disimpan ada di docs/macro-benchmark.txt.
"""
import hashlib, http.client, json, secrets, statistics, struct, sys, time, zlib

PORT = int(sys.argv[1])
REPEATS = int(sys.argv[2]) if len(sys.argv) > 2 else 30
KIB = 1024


class Client:
    def __init__(self):
        self.cookie = None

    def req(self, method, path, body=None, headers=None):
        conn = http.client.HTTPConnection("127.0.0.1", PORT, timeout=900)
        h = dict(headers or {})
        if self.cookie:
            h["Cookie"] = self.cookie
        start = time.perf_counter()
        conn.request(method, "/api" + path, body=body, headers=h)
        resp = conn.getresponse()
        data = resp.read()
        elapsed = time.perf_counter() - start
        self.last_headers = {k.lower(): v for k, v in resp.getheaders()}
        cookie = resp.getheader("Set-Cookie")
        if cookie:
            self.cookie = cookie.split(";")[0]
        conn.close()
        return resp.status, data, elapsed

    def json(self, method, path, payload):
        return self.req(method, path, json.dumps(payload).encode(), {"Content-Type": "application/json"})


def multipart(parts):
    boundary = "----bench" + secrets.token_hex(8)
    out = bytearray()
    for name, filename, ctype, data in parts:
        out += f"--{boundary}\r\n".encode()
        disposition = f'Content-Disposition: form-data; name="{name}"' + (f'; filename="{filename}"' if filename else "")
        out += (disposition + "\r\n").encode()
        if ctype:
            out += f"Content-Type: {ctype}\r\n".encode()
        out += b"\r\n" + data + b"\r\n"
    out += f"--{boundary}--\r\n".encode()
    return bytes(out), {"Content-Type": f"multipart/form-data; boundary={boundary}"}


def noise_png(target):
    """PNG berisi derau acak (tidak terkompresi), ukuran file sekitar target byte."""
    width = 1000
    height = max(1, target // (width * 3 + 1))
    rows = b"".join(b"\x00" + secrets.token_bytes(width * 3) for _ in range(height))

    def chunk(kind, data):
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data))

    return (b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(rows, 0)) + chunk(b"IEND", b""))


def stats(samples):
    ms = sorted(s * 1000 for s in samples)
    return f"{statistics.median(ms):10.1f} {ms[0]:10.1f} {ms[-1]:10.1f}"


def must(status, data, expected, what):
    if status != expected:
        raise SystemExit(f"{what}: HTTP {status}: {data[:300]!r}")


def new_user(prefix):
    client = Client()
    name = prefix + secrets.token_hex(3)
    must(*client.json("POST", "/users/register", {"name": name, "username": name, "password": "bench-password-1"})[:2], 201, "register")
    must(*client.json("POST", "/users/login", {"username": name, "password": "bench-password-1"})[:2], 200, "login")
    return client


def mib(n):
    return n / (1 << 20)


print(f"repeats={REPEATS}; times in ms (median / min / max) over the repeats; wall time seen by the client\n")

# ---- 1. file umum (dokumen, video): tiga cipher, dibaca per cipher lewat ?algorithm= ----
client = new_user("files")
ALGORITHMS = ("aes", "des", "rc4")
print("FILE UPLOAD (POST /api/files): server-reported cipher time in ms, and client wall time (median/min/max of 3 uploads)")
print(f"{'input':<14}{'bytes':>9} | {'wall':>8}{'min':>8}{'max':>8} | " + " | ".join(f"{a} enc{'':>2}{a} dec" for a in ALGORITHMS))
inputs = [("random-16KiB", secrets.token_bytes(16 * KIB)), ("random-256KiB", secrets.token_bytes(256 * KIB)),
          ("random-1MiB", secrets.token_bytes(KIB * KIB)), ("text-1MiB", (b"pasien=Budi;nik=3578012345670001;dosis=5mg\n" * 30000)[:KIB * KIB]),
          ("random-4MiB", secrets.token_bytes(4 * KIB * KIB))]
uploaded = []
for label, data in inputs:
    walls, first = [], None
    for _ in range(3):
        body, headers = multipart([("file", f"{label}.bin", "application/octet-stream", data)])
        status, resp, elapsed = client.req("POST", "/files", body, headers)
        must(status, resp, 201, f"upload {label}")
        meta = json.loads(resp)
        meta = meta.get("file", meta)
        first = first or meta
        walls.append(elapsed)
    cols = " | ".join(f"{first[a + '_encrypt_time_ns']/1e6:>8.1f}{first[a + '_decrypt_time_ns']/1e6:>8.1f}" for a in ALGORITHMS)
    print(f"{label:<14}{len(data):>9} | {statistics.median(walls)*1000:>8.1f}{min(walls)*1000:>8.1f}{max(walls)*1000:>8.1f} | {cols}")
    uploaded.append((label, data, first["id"]))

print(f"\nFILE DOWNLOAD (POST /api/files/:id/decrypt?algorithm=...): {REPEATS} repeats, {max(5, REPEATS // 3)} for 4 MiB, bytes verified by SHA-256")
print("wall = client time in ms; srv = X-Decryption-Time-Ns of the selected cipher in ms (median)")
print(f"{'input':<14}{'bytes':>9} {'algo':<5} | {'wall med':>9}{'min':>9}{'max':>9} | {'srv med':>9} | MB/s wall")
for label, data, file_id in uploaded:
    reps = REPEATS if len(data) <= KIB * KIB else max(5, REPEATS // 3)
    expected = hashlib.sha256(data).digest()
    for algorithm in ALGORITHMS:
        walls, server = [], []
        for _ in range(reps):
            status, resp, elapsed = client.req("POST", f"/files/{file_id}/decrypt?algorithm={algorithm}")
            must(status, resp, 200, f"decrypt {label} {algorithm}")
            if hashlib.sha256(resp).digest() != expected:
                raise SystemExit(f"{label} {algorithm}: downloaded bytes differ from the original")
            if client.last_headers.get("x-decryption-algorithm") != algorithm:
                raise SystemExit(f"{label}: server answered with algorithm {client.last_headers.get('x-decryption-algorithm')}")
            walls.append(elapsed)
            server.append(int(client.last_headers["x-decryption-time-ns"]) / 1e9)
        print(f"{label:<14}{len(data):>9} {algorithm:<5} | {stats(walls)} | {statistics.median(server)*1000:>9.1f} | {mib(len(data))/statistics.median(walls):.2f}")
print()

# ---- 2. KTP: seluruh field dan foto dienkripsi tiga cipher, dibaca dengan dekripsi ketiganya ----
print("KTP (POST then GET /api/patients/me/ktp[/photo]; every read decrypts AES, DES and RC4 and compares them)")
print(f"{'photo':<16}{'bytes':>9} | {'create ms':>10} | {'GET ktp ms':>10}{'min':>11}{'max':>11} | {'GET photo ms':>12}{'min':>11}{'max':>11} | MB/s")
ktp_json = {"full_name": "Pasien Sintetis", "birth_place": "Kota Contoh", "birth_date": "2005-01-01", "gender": "Laki-laki",
            "blood_type": "O", "address": "Jalan Contoh 1", "rt": "001", "rw": "002", "village": "Desa Contoh",
            "district": "Kecamatan Contoh", "religion": "Islam", "marital_status": "Belum Kawin", "occupation": "Pelajar",
            "citizenship": "WNI"}
for target in (256 * KIB, KIB * KIB, 4 * KIB * KIB):
    client = new_user("ktp")
    must(*client.json("POST", "/patients/me/profile", {"weight_kg": 65.5, "height_cm": 170, "age": 21, "nik": "3578012345670001", "bpjs_number": "0000000000001"})[:2], 201, "profile")
    photo = noise_png(target)
    body, headers = multipart([("data", None, None, json.dumps(ktp_json).encode()), ("photo", "ktp.png", "image/png", photo)])
    status, resp, create = client.req("POST", "/patients/me/ktp", body, headers)
    must(status, resp, 201, "create ktp")
    texts, photos = [], []
    expected = hashlib.sha256(photo).digest()
    for _ in range(max(5, REPEATS // 3) if target > KIB * KIB else REPEATS):
        status, resp, elapsed = client.req("GET", "/patients/me/ktp")
        must(status, resp, 200, "get ktp")
        texts.append(elapsed)
        status, resp, elapsed = client.req("GET", "/patients/me/ktp/photo")
        must(status, resp, 200, "get photo")
        if hashlib.sha256(resp).digest() != expected:
            raise SystemExit("photo bytes differ from the original")
        photos.append(elapsed)
    print(f"{'png-' + str(len(photo) // KIB) + 'KiB':<16}{len(photo):>9} | {create*1000:>10.1f} | {stats(texts)} | {stats(photos):>34} | {mib(len(photo))/statistics.median(photos):.2f}")
print("(photo bytes verified by SHA-256 on every read)")
