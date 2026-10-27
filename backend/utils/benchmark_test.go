package utils

// Benchmark mikro AES, DES, RC4 (jalankan dari folder backend):
//
//	go test ./utils -run '^$' -bench 'Raw' -benchtime=3x -count=10 | tee raw.txt
//	go test ./utils -run '^$' -bench 'Envelopes' -benchtime=3x -count=10 | tee envelopes.txt
//	benchstat raw.txt   (go install golang.org/x/perf/cmd/benchstat@latest)
//
// Raw mengukur cipher dan mode dengan kunci disiapkan di luar pengukuran; Envelopes mengukur fungsi yang
// dipakai pipeline (nonce, turunan kunci, cipher, HMAC). Datanya acak dengan seed tetap karena kecepatan
// cipher tidak bergantung pada isi data. Tutup program lain dan pakai daya AC agar hasil stabil.

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/rc4"
	"fmt"
	"math/rand/v2"
	"testing"
)

var benchSizes = []int{16 << 10, 256 << 10, 1 << 20}

// benchSink menahan hasil agar compiler tidak membuang pemanggilan yang diukur.
var benchSink []byte

type benchCase struct {
	name string
	// prepare mengubah data sebelum pengukuran dimulai, misalnya mengenkripsinya dulu untuk kasus dekripsi.
	prepare func(data []byte) []byte
	op      func(input []byte) []byte
}

func benchData(size int) []byte {
	data := make([]byte, size)
	rand.NewChaCha8([32]byte{9}).Read(data)
	return data
}

func runBenchmarkCases(b *testing.B, cases []benchCase) {
	for _, c := range cases {
		for _, size := range benchSizes {
			b.Run(fmt.Sprintf("%s/%dKiB", c.name, size>>10), func(b *testing.B) {
				input := benchData(size)
				if c.prepare != nil {
					input = c.prepare(input)
				}
				b.SetBytes(int64(size))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					benchSink = c.op(input)
				}
			})
		}
	}
}

// aesBlockFunc dan desBlockFunc menyiapkan kunci sekali, lalu mengembalikan enkripsi satu blok untuk CTRXor/cbcEncrypt.
func aesBlockFunc(key [16]byte) func([]byte) []byte {
	prepared := prepareAES128(key)
	return func(block []byte) []byte {
		out := prepared.encrypt([16]byte(block))
		return out[:]
	}
}

func desBlockFunc(key []byte) func([]byte) []byte {
	return DESBlockFunc(DESSubKeyMaker(DESBytesToBits(key)))
}

// cbcEncrypt adalah CBC dengan padding PKCS7, hanya arah enkripsi, untuk mengukur biaya mode CBC
// pada cipher yang sama dengan CTR. Tidak dipakai di kode produksi.
func cbcEncrypt(encrypt func([]byte) []byte, blockSize int, iv, plaintext []byte) []byte {
	padding := blockSize - len(plaintext)%blockSize
	padded := append(bytes.Clone(plaintext), bytes.Repeat([]byte{byte(padding)}, padding)...)
	output := make([]byte, 0, len(padded))
	previous := iv
	for offset := 0; offset < len(padded); offset += blockSize {
		block := make([]byte, blockSize)
		for i := range block {
			block[i] = padded[offset+i] ^ previous[i]
		}
		previous = encrypt(block)
		output = append(output, previous...)
	}
	return output
}

func BenchmarkRawModes(b *testing.B) {
	random := rand.NewChaCha8([32]byte{8})
	var aesKey [16]byte
	desKey, rc4Key := make([]byte, 8), make([]byte, 16)
	random.Read(aesKey[:])
	random.Read(desKey)
	random.Read(rc4Key)

	aesBlock, err := aes.NewCipher(aesKey[:])
	if err != nil {
		b.Fatal(err)
	}
	desBlock, err := des.NewCipher(desKey)
	if err != nil {
		b.Fatal(err)
	}
	aesEncrypt, desEncrypt := aesBlockFunc(aesKey), desBlockFunc(desKey)
	iv8, iv16 := make([]byte, 8), make([]byte, 16)

	stdlibCTR := func(block cipher.Block) func([]byte) []byte {
		return func(in []byte) []byte {
			out := make([]byte, len(in))
			cipher.NewCTR(block, counterIV(block.BlockSize(), desCTRStart)).XORKeyStream(out, in)
			return out
		}
	}

	runBenchmarkCases(b, []benchCase{
		{name: "AES-CTR", op: func(in []byte) []byte { return CTRXor(aesEncrypt, 16, desCTRStart, in) }},
		{name: "AES-CBC", op: func(in []byte) []byte { return cbcEncrypt(aesEncrypt, 16, iv16, in) }},
		{name: "DES-CTR", op: func(in []byte) []byte { return CTRXor(desEncrypt, 8, desCTRStart, in) }},
		{name: "DES-CBC", op: func(in []byte) []byte { return cbcEncrypt(desEncrypt, 8, iv8, in) }},
		{name: "RC4", op: func(in []byte) []byte { return RC4Xor(rc4Key, in, rc4FileDrop) }},
		{name: "stdlib-AES-CTR", op: stdlibCTR(aesBlock)},
		{name: "stdlib-DES-CTR", op: stdlibCTR(desBlock)},
		{name: "stdlib-RC4", op: func(in []byte) []byte {
			stream, err := rc4.NewCipher(rc4Key)
			if err != nil {
				panic(err)
			}
			stream.XORKeyStream(make([]byte, rc4FileDrop), make([]byte, rc4FileDrop))
			out := make([]byte, len(in))
			stream.XORKeyStream(out, in)
			return out
		}},
	})
}

func BenchmarkEnvelopes(b *testing.B) {
	var key [32]byte
	for i := range key {
		key[i] = byte(i*13 + 1)
	}
	apply := func(f func([]byte, [32]byte) ([]byte, error)) func([]byte) []byte {
		return func(in []byte) []byte {
			out, err := f(in, key)
			if err != nil {
				panic(err)
			}
			return out
		}
	}

	runBenchmarkCases(b, []benchCase{
		{name: "AES1-encrypt", op: apply(EncryptAESFile)},
		{name: "AES1-decrypt", prepare: apply(EncryptAESFile), op: apply(DecryptAESFile)},
		{name: "DES1-encrypt", op: apply(EncryptDESFile)},
		{name: "DES1-decrypt", prepare: apply(EncryptDESFile), op: apply(DecryptDESFile)},
		{name: "RC41-encrypt", op: apply(EncryptRC4File)},
		{name: "RC41-decrypt", prepare: apply(EncryptRC4File), op: apply(DecryptRC4File)},
	})
}

// Alat ukur CBC harus benar agar perbandingan CBC dan CTR bermakna.
func TestCBCEncryptMatchesStdlib(t *testing.T) {
	random := rand.NewChaCha8([32]byte{10})
	var aesKey [16]byte
	desKey := make([]byte, 8)
	random.Read(aesKey[:])
	random.Read(desKey)
	aesBlock, err := aes.NewCipher(aesKey[:])
	if err != nil {
		t.Fatal(err)
	}
	desBlock, err := des.NewCipher(desKey)
	if err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		block   cipher.Block
		encrypt func([]byte) []byte
	}{
		"AES": {aesBlock, aesBlockFunc(aesKey)},
		"DES": {desBlock, desBlockFunc(desKey)},
	} {
		size := tc.block.BlockSize()
		iv := make([]byte, size)
		random.Read(iv)

		for _, length := range []int{0, 1, size - 1, size, size + 1, 100} {
			plaintext := make([]byte, length)
			random.Read(plaintext)
			padding := size - length%size
			padded := append(bytes.Clone(plaintext), bytes.Repeat([]byte{byte(padding)}, padding)...)
			want := make([]byte, len(padded))
			cipher.NewCBCEncrypter(tc.block, iv).CryptBlocks(want, padded)

			if got := cbcEncrypt(tc.encrypt, size, iv, plaintext); !bytes.Equal(got, want) {
				t.Fatalf("%s length %d: got %x, want %x", name, length, got, want)
			}
		}
	}
}
