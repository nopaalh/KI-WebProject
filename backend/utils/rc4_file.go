package utils

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
)

const (
	rc4FileMagic      = "RC41"
	rc4FileNonceSize  = 16
	rc4FileHeaderSize = len(rc4FileMagic) + rc4FileNonceSize
	rc4FileEncLabel   = "RC41/encryption"
	rc4FileMACLabel   = "RC41/authentication"
	// rc4FileDrop membuang 1536 byte awal keystream RC4 (RFC 4345, arcfour128) karena byte awalnya bias.
	rc4FileDrop = 1536
)

var (
	errRC4FileFormat         = errors.New("RC4 file: invalid envelope")
	errRC4FileAuthentication = errors.New("RC4 file: authentication failed")
)

// EncryptRC4File returns magic "RC41" || random 16-byte nonce || RC4 ciphertext ||
// HMAC-SHA256(magic || nonce || ciphertext). There is no padding, so the ciphertext is as long as the plaintext.
// The RC4 key is the first 16 bytes of HMAC-SHA256(key, "RC41/encryption" || nonce), so every file gets its
// own keystream; reusing one RC4 key would leak the XOR of two plaintexts. The first 1536 keystream bytes are
// discarded. The MAC key is HMAC-SHA256(key, "RC41/authentication").
// RC4 is cryptographically broken and is used here only for the assignment comparison.
func EncryptRC4File(plaintext []byte, key [32]byte) ([]byte, error) {
	if len(plaintext) > int(^uint(0)>>1)-rc4FileHeaderSize-sha256.Size {
		return nil, errors.New("RC4 file: plaintext too large")
	}
	bodyEnd := rc4FileHeaderSize + len(plaintext)
	encrypted := make([]byte, bodyEnd+sha256.Size)
	copy(encrypted, rc4FileMagic)
	nonce := encrypted[len(rc4FileMagic):rc4FileHeaderSize]
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("RC4 file: generate nonce: %w", err)
	}

	encKey, macKey := deriveRC4FileKeys(key, nonce)
	copy(encrypted[rc4FileHeaderSize:], RC4Xor(encKey[:], plaintext, rc4FileDrop))

	mac := hmac.New(sha256.New, macKey[:])
	mac.Write(encrypted[:bodyEnd])
	copy(encrypted[bodyEnd:], mac.Sum(nil))
	return encrypted, nil
}

// DecryptRC4File validates the envelope and authenticates it before performing
// any RC4 decryption. Errors never return partial plaintext.
func DecryptRC4File(encrypted []byte, key [32]byte) ([]byte, error) {
	if len(encrypted) < rc4FileHeaderSize+sha256.Size {
		return nil, errRC4FileFormat
	}
	bodyEnd := len(encrypted) - sha256.Size
	if string(encrypted[:len(rc4FileMagic)]) != rc4FileMagic {
		return nil, errRC4FileFormat
	}

	nonce := encrypted[len(rc4FileMagic):rc4FileHeaderSize]
	encKey, macKey := deriveRC4FileKeys(key, nonce)
	mac := hmac.New(sha256.New, macKey[:])
	mac.Write(encrypted[:bodyEnd])
	if !hmac.Equal(encrypted[bodyEnd:], mac.Sum(nil)) {
		return nil, errRC4FileAuthentication
	}

	return RC4Xor(encKey[:], encrypted[rc4FileHeaderSize:bodyEnd], rc4FileDrop), nil
}

func deriveRC4FileKeys(key [32]byte, nonce []byte) (encKey [16]byte, macKey [32]byte) {
	mac := hmac.New(sha256.New, key[:])
	mac.Write([]byte(rc4FileEncLabel))
	mac.Write(nonce)
	copy(encKey[:], mac.Sum(nil))
	mac.Reset()
	mac.Write([]byte(rc4FileMACLabel))
	copy(macKey[:], mac.Sum(nil))
	return encKey, macKey
}
