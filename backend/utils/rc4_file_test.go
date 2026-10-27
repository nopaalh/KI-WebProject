package utils

import (
	"bytes"
	"crypto/rc4"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
)

func rc4FileTestKey() (key [32]byte) {
	for i := range key {
		key[i] = byte(i*11 + 5)
	}
	return key
}

func TestRC4FileRoundTrip(t *testing.T) {
	key := rc4FileTestKey()
	for _, size := range []int{0, 1, 7, 8, 9, 1000, 64*1024 + 7} {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			plaintext := make([]byte, size)
			for i := range plaintext {
				plaintext[i] = byte(i*37 + i/256)
			}
			original := bytes.Clone(plaintext)

			encrypted, err := EncryptRC4File(plaintext, key)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(plaintext, original) {
				t.Fatal("encryption modified the input")
			}
			if want := 4 + 16 + size + sha256.Size; len(encrypted) != want || string(encrypted[:4]) != "RC41" {
				t.Fatalf("invalid envelope: length %d, want %d; magic %q", len(encrypted), want, encrypted[:4])
			}

			got, err := DecryptRC4File(encrypted, key)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, original) {
				t.Fatal("round trip does not match the original")
			}
		})
	}
}

// Kunci RC4 yang dipakai ulang akan membocorkan XOR dua plaintext, jadi tiap file harus punya keystream sendiri.
func TestRC4FileUsesFreshNonceAndKeyPerFile(t *testing.T) {
	key := rc4FileTestKey()
	plaintext := bytes.Repeat([]byte("rekam medis "), 4)

	first, err := EncryptRC4File(plaintext, key)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EncryptRC4File(plaintext, key)
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

// Badan envelope harus RC4 biasa dengan 1536 byte awal dibuang, jadi stdlib bisa mendekripsinya.
func TestRC4FileInteroperatesWithStandardLibrary(t *testing.T) {
	key := rc4FileTestKey()
	plaintext := []byte("hasil lab pasien, 21 byte lebih")

	encrypted, err := EncryptRC4File(plaintext, key)
	if err != nil {
		t.Fatal(err)
	}

	encKey, _ := deriveRC4FileKeys(key, encrypted[4:20])
	stdlib, err := rc4.NewCipher(encKey[:])
	if err != nil {
		t.Fatal(err)
	}
	stdlib.XORKeyStream(make([]byte, rc4FileDrop), make([]byte, rc4FileDrop))
	got := make([]byte, len(plaintext))
	stdlib.XORKeyStream(got, encrypted[20:len(encrypted)-sha256.Size])

	if !bytes.Equal(got, plaintext) {
		t.Fatalf("stdlib decrypted %q, want %q", got, plaintext)
	}
}

func TestRC4FileRejectsTamperingAndWrongKey(t *testing.T) {
	key := rc4FileTestKey()
	encrypted, err := EncryptRC4File([]byte("dosis obat 5 mg per hari"), key)
	if err != nil {
		t.Fatal(err)
	}

	// Mengubah satu bit di nonce, ciphertext, atau MAC harus gagal di autentikasi.
	for _, index := range []int{4, 19, 20, len(encrypted) - sha256.Size - 1, len(encrypted) - 1} {
		tampered := bytes.Clone(encrypted)
		tampered[index] ^= 0x01
		if got, err := DecryptRC4File(tampered, key); !errors.Is(err, errRC4FileAuthentication) || got != nil {
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
		if got, err := DecryptRC4File(input, key); !errors.Is(err, errRC4FileFormat) || got != nil {
			t.Fatalf("%s: got %q, %v; want format error", name, got, err)
		}
	}

	wrongKey := key
	wrongKey[0] ^= 0x01
	if got, err := DecryptRC4File(encrypted, wrongKey); !errors.Is(err, errRC4FileAuthentication) || got != nil {
		t.Fatalf("wrong key: got %q, %v; want authentication error", got, err)
	}
}
