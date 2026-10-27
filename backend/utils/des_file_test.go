package utils

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
)

func desFileTestKey() (key [32]byte) {
	for i := range key {
		key[i] = byte(i*7 + 3)
	}
	return key
}

func TestDESFileRoundTrip(t *testing.T) {
	key := desFileTestKey()
	for _, size := range []int{0, 1, 7, 8, 9, 1000} {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			plaintext := make([]byte, size)
			for i := range plaintext {
				plaintext[i] = byte(i*37 + i/256)
			}
			original := bytes.Clone(plaintext)

			encrypted, err := EncryptDESFile(plaintext, key)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(plaintext, original) {
				t.Fatal("encryption modified the input")
			}
			if want := 4 + 16 + size + sha256.Size; len(encrypted) != want || string(encrypted[:4]) != "DES1" {
				t.Fatalf("invalid envelope: length %d, want %d; magic %q", len(encrypted), want, encrypted[:4])
			}

			got, err := DecryptDESFile(encrypted, key)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, original) {
				t.Fatalf("round trip = %x, want %x", got, original)
			}
		})
	}
}

func TestDESFileUsesFreshNonceAndKeyPerFile(t *testing.T) {
	key := desFileTestKey()
	plaintext := bytes.Repeat([]byte("rekam medis "), 4)

	first, err := EncryptDESFile(plaintext, key)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EncryptDESFile(plaintext, key)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(first[4:20], second[4:20]) {
		t.Fatal("nonce repeated")
	}
	if bytes.Equal(first[20:len(first)-sha256.Size], second[20:len(second)-sha256.Size]) {
		t.Fatal("same plaintext produced the same ciphertext")
	}
	if bytes.Equal(first[len(first)-sha256.Size:], second[len(second)-sha256.Size:]) {
		t.Fatal("MAC repeated")
	}
}

// The envelope body must be plain DES-CTR under the derived key, so the standard library can decrypt it.
func TestDESFileInteroperatesWithStandardLibrary(t *testing.T) {
	key := desFileTestKey()
	plaintext := []byte("hasil lab pasien, 21 byte lebih")

	encrypted, err := EncryptDESFile(plaintext, key)
	if err != nil {
		t.Fatal(err)
	}

	encKey, _ := deriveDESFileKeys(key, encrypted[4:20])
	block, err := des.NewCipher(encKey[:])
	if err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(plaintext))
	cipher.NewCTR(block, counterIV(8, desCTRStart)).XORKeyStream(got, encrypted[20:len(encrypted)-sha256.Size])

	if !bytes.Equal(got, plaintext) {
		t.Fatalf("stdlib decrypted %q, want %q", got, plaintext)
	}
}

func TestDESFileRejectsTamperingAndWrongKey(t *testing.T) {
	key := desFileTestKey()
	encrypted, err := EncryptDESFile([]byte("dosis obat 5 mg per hari"), key)
	if err != nil {
		t.Fatal(err)
	}

	// Mengubah satu bit di nonce, ciphertext, atau MAC harus gagal di autentikasi.
	for _, index := range []int{4, 19, 20, len(encrypted) - sha256.Size - 1, len(encrypted) - 1} {
		tampered := bytes.Clone(encrypted)
		tampered[index] ^= 0x01
		if got, err := DecryptDESFile(tampered, key); !errors.Is(err, errDESFileAuthentication) || got != nil {
			t.Fatalf("byte %d: got %q, %v; want authentication error", index, got, err)
		}
	}

	// Magic yang salah dan envelope yang terpotong gagal di format.
	badMagic := bytes.Clone(encrypted)
	badMagic[0] ^= 0x01
	for name, input := range map[string][]byte{
		"magic":     badMagic,
		"truncated": encrypted[:20+sha256.Size-1],
		"empty":     nil,
	} {
		if got, err := DecryptDESFile(input, key); !errors.Is(err, errDESFileFormat) || got != nil {
			t.Fatalf("%s: got %q, %v; want format error", name, got, err)
		}
	}

	wrongKey := key
	wrongKey[0] ^= 0x01
	if got, err := DecryptDESFile(encrypted, wrongKey); !errors.Is(err, errDESFileAuthentication) || got != nil {
		t.Fatalf("wrong key: got %q, %v; want authentication error", got, err)
	}
}
