package utils

// Analisis ciphertext envelope AES1, DES1, dan RC41. Dua tes pertama cepat dan ikut dijalankan CI. Laporan di akhir
// berat (dekripsi diulang) sehingga hanya jalan bila diminta, dari folder backend:
//
//	ANALYSIS_REPORT=1 go test ./utils -run Report -v -count=1 -timeout 30m
//
// Variabel opsional: ANALYSIS_REPEATS (default 30) dan ANALYSIS_FILES (daftar path dipisah ; di Windows, : di Linux).
// Path relatif terhadap folder utils karena go test berjalan di sana, misalnya ../../frontend/public/images/portal-mark.png.
// PowerShell: $env:ANALYSIS_REPORT=1; go test ... Waktu yang dilaporkan hanya kriptografi, tanpa HTTP, JWT, atau disk.

import (
	"bytes"
	"fmt"
	"math"
	"math/bits"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"
)

type analysisEnvelope struct {
	name    string
	encrypt func([]byte, [32]byte) ([]byte, error)
	decrypt func([]byte, [32]byte) ([]byte, error)
	// overhead adalah selisih panjang envelope dan plaintext yang diharapkan.
	overhead func(plain int) int
}

// AES1 menambah padding PKCS7 (1 sampai 16 byte); DES1 dan RC41 hanya magic, nonce, dan HMAC.
var analysisEnvelopes = []analysisEnvelope{
	{"AES1", EncryptAESFile, DecryptAESFile, func(n int) int { return 4 + 16 + (16 - n%16) + 32 }},
	{"DES1", EncryptDESFile, DecryptDESFile, func(int) int { return 4 + 16 + 32 }},
	{"RC41", EncryptRC4File, DecryptRC4File, func(int) int { return 4 + 16 + 32 }},
}

func analysisKey() (key [32]byte) {
	for i := range key {
		key[i] = byte(i*7 + 3)
	}
	return key
}

// byteEntropy menghitung entropi Shannon dalam bit per byte; maksimum 8 untuk data acak.
func byteEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}
	var counts [256]int
	for _, b := range data {
		counts[b]++
	}
	entropy := 0.0
	for _, n := range counts {
		if n > 0 {
			p := float64(n) / float64(len(data))
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

// analysisText membuat data berentropi rendah dan berulang, mirip dokumen teks atau CSV.
func analysisText(size int) []byte {
	record := []byte("pasien=Budi Santoso;nik=3578012345670001;diagnosa=hipertensi;dosis=5 mg per hari\n")
	return bytes.Repeat(record, size/len(record)+1)[:size]
}

// analysisRandom mewakili data yang sudah padat, seperti JPEG, PNG, atau video terkompresi.
func analysisRandom(size int) []byte {
	data := make([]byte, size)
	rand.NewChaCha8([32]byte{11}).Read(data)
	return data
}

func differingBytes(a, b []byte) float64 {
	differ := 0
	for i := range a {
		if a[i] != b[i] {
			differ++
		}
	}
	return float64(differ) / float64(len(a))
}

func differingBits(a, b []byte) int {
	total := 0
	for i := range a {
		total += bits.OnesCount8(a[i] ^ b[i])
	}
	return total
}

func distinctBlocks(data []byte, size int) (distinct, total int) {
	seen := map[string]struct{}{}
	for offset := 0; offset+size <= len(data); offset += size {
		seen[string(data[offset:offset+size])] = struct{}{}
		total++
	}
	return len(seen), total
}

func TestEnvelopeCiphertextProperties(t *testing.T) {
	key := analysisKey()

	for _, envelope := range analysisEnvelopes {
		t.Run(envelope.name, func(t *testing.T) {
			// Overhead tepat untuk panjang di sekitar batas blok.
			for _, size := range []int{0, 1, 7, 8, 15, 16, 17, 1000} {
				plaintext := analysisRandom(size)
				encrypted, err := envelope.encrypt(plaintext, key)
				if err != nil {
					t.Fatal(err)
				}
				if got, want := len(encrypted)-size, envelope.overhead(size); got != want {
					t.Fatalf("size %d: overhead %d, want %d", size, got, want)
				}
				decrypted, err := envelope.decrypt(encrypted, key)
				if err != nil || !bytes.Equal(decrypted, plaintext) {
					t.Fatalf("size %d: round trip failed: %v", size, err)
				}
			}

			// Plaintext berpola dan berentropi rendah menghasilkan ciphertext yang tampak acak.
			plaintext := analysisText(64 << 10)
			if entropy := byteEntropy(plaintext); entropy > 6 {
				t.Fatalf("test data is not low entropy: %.2f", entropy)
			}
			first, err := envelope.encrypt(plaintext, key)
			if err != nil {
				t.Fatal(err)
			}
			second, err := envelope.encrypt(plaintext, key)
			if err != nil {
				t.Fatal(err)
			}
			body1, body2 := first[20:len(first)-32], second[20:len(second)-32]
			if entropy := byteEntropy(body1); entropy < 7.99 {
				t.Fatalf("ciphertext entropy %.4f, want at least 7.99", entropy)
			}

			// Nonce acak: enkripsi kedua dari plaintext yang sama berbeda hampir di seluruh byte (harapan 255/256).
			if bytes.Equal(first[4:20], second[4:20]) {
				t.Fatal("nonce repeated")
			}
			if fraction := differingBytes(body1, body2); fraction < 0.99 {
				t.Fatalf("only %.4f of ciphertext bytes differ between two encryptions", fraction)
			}

			// Blok plaintext identik tidak boleh menghasilkan blok ciphertext identik (tidak seperti ECB).
			repeated, err := envelope.encrypt(bytes.Repeat([]byte("0123456789abcdef"), 1024), key)
			if err != nil {
				t.Fatal(err)
			}
			if distinct, total := distinctBlocks(repeated[20:len(repeated)-32], 16); distinct != total {
				t.Fatalf("%d of %d ciphertext blocks are distinct, want all", distinct, total)
			}
		})
	}
}

// Satu bit plaintext yang diubah menunjukkan beda CBC dan mode aliran: CTR dan RC4 hanya membalik satu bit ciphertext
// (bisa dimanipulasi tanpa HMAC), sedangkan CBC menyebar ke blok-blok berikutnya.
func TestSingleBitFlipDiffusion(t *testing.T) {
	random := rand.NewChaCha8([32]byte{12})
	var aesKey [16]byte
	desKey, rc4Key := make([]byte, 8), make([]byte, 16)
	random.Read(aesKey[:])
	random.Read(desKey)
	random.Read(rc4Key)
	aesEncrypt, desEncrypt := aesBlockFunc(aesKey), desBlockFunc(desKey)

	plaintext := analysisRandom(4096)
	flipped := bytes.Clone(plaintext)
	flipped[3] ^= 0x01
	iv := make([]byte, 16)

	for name, encrypt := range map[string]func([]byte) []byte{
		"AES-CTR": func(in []byte) []byte { return CTRXor(aesEncrypt, 16, desCTRStart, in) },
		"DES-CTR": func(in []byte) []byte { return CTRXor(desEncrypt, 8, desCTRStart, in) },
		"RC4":     func(in []byte) []byte { return RC4Xor(rc4Key, in, rc4FileDrop) },
	} {
		if got := differingBits(encrypt(plaintext), encrypt(flipped)); got != 1 {
			t.Errorf("%s: %d ciphertext bits changed, want exactly 1", name, got)
		}
	}

	cbcA, cbcB := cbcEncrypt(aesEncrypt, 16, iv, plaintext), cbcEncrypt(aesEncrypt, 16, iv, flipped)
	if fraction := differingBytes(cbcA, cbcB); fraction < 0.99 {
		t.Errorf("AES-CBC: only %.4f of ciphertext bytes changed after a one-bit change", fraction)
	}
	if bytes.Equal(cbcA[:16], cbcB[:16]) {
		t.Error("AES-CBC: first block should change")
	}
}

func TestEnvelopeAnalysisReport(t *testing.T) {
	if os.Getenv("ANALYSIS_REPORT") == "" {
		t.Skip("set ANALYSIS_REPORT=1 to run the long ciphertext and repeated-decryption report")
	}
	repeats := 30
	if value := os.Getenv("ANALYSIS_REPEATS"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			t.Fatalf("ANALYSIS_REPEATS must be a positive integer, got %q", value)
		}
		repeats = n
	}

	type input struct {
		name string
		data []byte
	}
	var inputs []input
	for _, size := range []int{16 << 10, 256 << 10, 1 << 20} {
		inputs = append(inputs,
			input{fmt.Sprintf("text-%dKiB", size>>10), analysisText(size)},
			input{fmt.Sprintf("random-%dKiB", size>>10), analysisRandom(size)})
	}
	for _, path := range filepath.SplitList(os.Getenv("ANALYSIS_FILES")) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, input{filepath.Base(path), data})
	}

	key := analysisKey()
	t.Logf("%-24s %-5s %10s %9s %8s %8s %12s %12s %12s", "input", "algo", "ct-bytes", "overhead", "H(plain)", "H(ct)", "enc-median", "dec-median", "dec-min/max")
	for _, in := range inputs {
		for _, envelope := range analysisEnvelopes {
			encrypted, err := envelope.encrypt(in.data, key)
			if err != nil {
				t.Fatal(err)
			}

			encTimes := make([]time.Duration, 0, repeats)
			decTimes := make([]time.Duration, 0, repeats)
			for range repeats {
				start := time.Now()
				if _, err := envelope.encrypt(in.data, key); err != nil {
					t.Fatal(err)
				}
				encTimes = append(encTimes, time.Since(start))

				start = time.Now()
				decrypted, err := envelope.decrypt(encrypted, key)
				decTimes = append(decTimes, time.Since(start))
				if err != nil || !bytes.Equal(decrypted, in.data) {
					t.Fatalf("%s %s: round trip failed: %v", in.name, envelope.name, err)
				}
			}
			slices.Sort(encTimes)
			slices.Sort(decTimes)

			t.Logf("%-24s %-5s %10d %9d %8.4f %8.4f %12v %12v %6v/%v",
				in.name, envelope.name, len(encrypted), len(encrypted)-len(in.data),
				byteEntropy(in.data), byteEntropy(encrypted[20:len(encrypted)-32]),
				encTimes[len(encTimes)/2].Round(time.Microsecond), decTimes[len(decTimes)/2].Round(time.Microsecond),
				decTimes[0].Round(time.Millisecond), decTimes[len(decTimes)-1].Round(time.Millisecond))
		}
	}
}
