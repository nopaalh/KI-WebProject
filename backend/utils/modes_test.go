package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"encoding/binary"
	"math/rand/v2"
	"testing"
)

// counterIV membuat blok counter awal untuk cipher.NewCTR dari stdlib: start di 8 byte terakhir, big-endian.
func counterIV(blockSize int, start uint64) []byte {
	iv := make([]byte, blockSize)
	binary.BigEndian.PutUint64(iv[blockSize-8:], start)
	return iv
}

func TestCTRXorMatchesStdlibDES(t *testing.T) {
	random := rand.NewChaCha8([32]byte{2})
	key := make([]byte, 8)
	random.Read(key)
	block, err := des.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	encrypt := DESBlockFunc(DESSubKeyMaker(DESBytesToBits(key)))

	// Panjang 0, 1, 7, 8, 9 menguji blok terakhir yang tidak penuh.
	for _, size := range []int{0, 1, 7, 8, 9, 64, 1000} {
		data := make([]byte, size)
		random.Read(data)

		want := make([]byte, size)
		cipher.NewCTR(block, counterIV(8, desCTRStart)).XORKeyStream(want, data)
		got := CTRXor(encrypt, 8, desCTRStart, data)

		if !bytes.Equal(got, want) {
			t.Fatalf("size %d: got %x, want %x", size, got, want)
		}
	}
}

func TestCTRXorMatchesStdlibAES(t *testing.T) {
	random := rand.NewChaCha8([32]byte{3})
	var key [16]byte
	random.Read(key[:])
	block, err := aes.NewCipher(key[:])
	if err != nil {
		t.Fatal(err)
	}
	prepared := prepareAES128(key)
	encrypt := func(b []byte) []byte {
		out := prepared.encrypt([16]byte(b))
		return out[:]
	}

	for _, size := range []int{0, 1, 15, 16, 17, 100} {
		data := make([]byte, size)
		random.Read(data)

		want := make([]byte, size)
		cipher.NewCTR(block, counterIV(16, desCTRStart)).XORKeyStream(want, data)
		got := CTRXor(encrypt, 16, desCTRStart, data)

		if !bytes.Equal(got, want) {
			t.Fatalf("size %d: got %x, want %x", size, got, want)
		}
	}
}

func TestCTRXorRoundTrip(t *testing.T) {
	encrypt := DESBlockFunc(DESSubKeyMaker(bitsFromHex("133457799BBCDFF1")))
	data := []byte("rekam medis pasien 007")
	original := bytes.Clone(data)

	encrypted := CTRXor(encrypt, 8, desCTRStart, data)

	if len(encrypted) != len(data) || bytes.Equal(encrypted, data) {
		t.Fatalf("ciphertext harus sama panjang dan berbeda dari plaintext: %x", encrypted)
	}
	if got := CTRXor(encrypt, 8, desCTRStart, encrypted); !bytes.Equal(got, original) {
		t.Fatalf("round trip = %q, want %q", got, original)
	}
	if !bytes.Equal(data, original) {
		t.Fatal("CTRXor mengubah input")
	}
}
