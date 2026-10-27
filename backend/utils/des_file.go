package utils

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
)

const (
	desFileMagic      = "DES1"
	desFileNonceSize  = 16
	desFileHeaderSize = len(desFileMagic) + desFileNonceSize
	desFileEncLabel   = "DES1/encryption"
	desFileMACLabel   = "DES1/authentication"
)

var (
	errDESFileFormat         = errors.New("DES file: invalid envelope")
	errDESFileAuthentication = errors.New("DES file: authentication failed")
)

// EncryptDESFile returns magic "DES1" || random 16-byte nonce || DES-CTR ciphertext ||
// HMAC-SHA256(magic || nonce || ciphertext). There is no padding, so the ciphertext is as long as the plaintext.
// The DES key is the first 8 bytes of HMAC-SHA256(key, "DES1/encryption" || nonce), so every file gets its
// own key and the fixed CTR start never repeats a keystream. The MAC key is HMAC-SHA256(key, "DES1/authentication").
// This uses educational, non-constant-time handmade DES, not production crypto.
func EncryptDESFile(plaintext []byte, key [32]byte) ([]byte, error) {
	if len(plaintext) > int(^uint(0)>>1)-desFileHeaderSize-sha256.Size {
		return nil, errors.New("DES file: plaintext too large")
	}
	bodyEnd := desFileHeaderSize + len(plaintext)
	encrypted := make([]byte, bodyEnd+sha256.Size)
	copy(encrypted, desFileMagic)
	nonce := encrypted[len(desFileMagic):desFileHeaderSize]
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("DES file: generate nonce: %w", err)
	}

	encKey, macKey := deriveDESFileKeys(key, nonce)
	copy(encrypted[desFileHeaderSize:], desFileCTR(encKey, plaintext))

	mac := hmac.New(sha256.New, macKey[:])
	mac.Write(encrypted[:bodyEnd])
	copy(encrypted[bodyEnd:], mac.Sum(nil))
	return encrypted, nil
}

// DecryptDESFile validates the envelope and authenticates it before performing
// any DES decryption. Errors never return partial plaintext.
// The handmade DES is educational and is not constant-time.
func DecryptDESFile(encrypted []byte, key [32]byte) ([]byte, error) {
	if len(encrypted) < desFileHeaderSize+sha256.Size {
		return nil, errDESFileFormat
	}
	bodyEnd := len(encrypted) - sha256.Size
	if string(encrypted[:len(desFileMagic)]) != desFileMagic {
		return nil, errDESFileFormat
	}

	nonce := encrypted[len(desFileMagic):desFileHeaderSize]
	encKey, macKey := deriveDESFileKeys(key, nonce)
	mac := hmac.New(sha256.New, macKey[:])
	mac.Write(encrypted[:bodyEnd])
	if !hmac.Equal(encrypted[bodyEnd:], mac.Sum(nil)) {
		return nil, errDESFileAuthentication
	}

	return desFileCTR(encKey, encrypted[desFileHeaderSize:bodyEnd]), nil
}

// desFileCTR runs DES-CTR with encKey; encryption and decryption are the same operation.
func desFileCTR(encKey [8]byte, data []byte) []byte {
	subkeys := DESSubKeyMaker(DESBytesToBits(encKey[:]))
	return CTRXor(DESBlockFunc(subkeys), 8, desCTRStart, data)
}

func deriveDESFileKeys(key [32]byte, nonce []byte) (encKey [8]byte, macKey [32]byte) {
	mac := hmac.New(sha256.New, key[:])
	mac.Write([]byte(desFileEncLabel))
	mac.Write(nonce)
	copy(encKey[:], mac.Sum(nil))
	mac.Reset()
	mac.Write([]byte(desFileMACLabel))
	copy(macKey[:], mac.Sum(nil))
	return encKey, macKey
}
