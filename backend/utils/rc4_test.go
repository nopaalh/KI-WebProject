package utils

import (
	"bytes"
	"crypto/rc4"
	"encoding/hex"
	"math/rand/v2"
	"testing"
)

func TestRC4XorKnownVectors(t *testing.T) {
	for _, vector := range []struct{ key, plain, cipher string }{
		{"Key", "Plaintext", "BBF316E8D940AF0AD3"},
		{"Wiki", "pedia", "1021BF0420"},
		{"Secret", "Attack at dawn", "45A01F645FC35B383552544B9BF5"},
	} {
		want, err := hex.DecodeString(vector.cipher)
		if err != nil {
			t.Fatal(err)
		}
		if got := RC4Xor([]byte(vector.key), []byte(vector.plain), 0); !bytes.Equal(got, want) {
			t.Fatalf("key %q: got %X, want %s", vector.key, got, vector.cipher)
		}
	}
}

// Kunci dan data acak (seed tetap) dibandingkan dengan crypto/rc4, dengan dan tanpa drop 1536.
func TestRC4XorMatchesStdlib(t *testing.T) {
	random := rand.NewChaCha8([32]byte{4})
	for _, keySize := range []int{1, 5, 16, 256} {
		for _, dataSize := range []int{0, 1, 255, 256, 257, 5000} {
			for _, drop := range []int{0, 1536} {
				key, data := make([]byte, keySize), make([]byte, dataSize)
				random.Read(key)
				random.Read(data)

				stdlib, err := rc4.NewCipher(key)
				if err != nil {
					t.Fatal(err)
				}
				stdlib.XORKeyStream(make([]byte, drop), make([]byte, drop))
				want := make([]byte, dataSize)
				stdlib.XORKeyStream(want, data)

				if got := RC4Xor(key, data, drop); !bytes.Equal(got, want) {
					t.Fatalf("key %d byte, data %d byte, drop %d: mismatch", keySize, dataSize, drop)
				}
			}
		}
	}
}

func TestRC4XorRoundTrip(t *testing.T) {
	key := []byte("kunci rahasia 16")
	data := []byte("rekam medis pasien 007")
	original := bytes.Clone(data)

	encrypted := RC4Xor(key, data, 1536)

	if len(encrypted) != len(data) || bytes.Equal(encrypted, data) {
		t.Fatalf("ciphertext harus sama panjang dan berbeda dari plaintext: %x", encrypted)
	}
	if got := RC4Xor(key, encrypted, 1536); !bytes.Equal(got, original) {
		t.Fatalf("round trip = %q, want %q", got, original)
	}
	if !bytes.Equal(data, original) {
		t.Fatal("RC4Xor mengubah input")
	}
}
