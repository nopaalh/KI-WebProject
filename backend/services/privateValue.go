package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"

	"project1/utils"
)

var ErrPrivateValueIntegrity = errors.New("private value failed integrity verification")

// encryptTriple menyimpan satu nilai privat sebagai amplop AES, DES, dan RC4, lalu memastikan ketiganya bisa didekripsi kembali menjadi nilai yang sama.
func encryptTriple(key [32]byte, plaintext []byte) (aes, des, rc4 []byte, err error) {
	if aes, err = utils.EncryptAESFile(plaintext, key); err != nil {
		return nil, nil, nil, fmt.Errorf("encrypt with AES: %w", err)
	}
	if des, err = utils.EncryptDESFile(plaintext, key); err != nil {
		return nil, nil, nil, fmt.Errorf("encrypt with DES: %w", err)
	}
	if rc4, err = utils.EncryptRC4File(plaintext, key); err != nil {
		return nil, nil, nil, fmt.Errorf("encrypt with RC4: %w", err)
	}
	decoded, err := decryptTriple(key, aes, des, rc4)
	if err != nil || !bytes.Equal(decoded, plaintext) {
		return nil, nil, nil, ErrPrivateValueIntegrity
	}
	return aes, des, rc4, nil
}

// decryptTriple mengembalikan nilai hanya jika ketiga amplop lolos tag dan isinya sama.
func decryptTriple(key [32]byte, aes, des, rc4 []byte) ([]byte, error) {
	plaintext, err := utils.DecryptAESFile(aes, key)
	if err != nil {
		return nil, ErrPrivateValueIntegrity
	}
	for _, copy := range []struct {
		data    []byte
		decrypt func([]byte, [32]byte) ([]byte, error)
	}{{des, utils.DecryptDESFile}, {rc4, utils.DecryptRC4File}} {
		decoded, err := copy.decrypt(copy.data, key)
		if err != nil || !bytes.Equal(decoded, plaintext) {
			return nil, ErrPrivateValueIntegrity
		}
	}
	return plaintext, nil
}

// usernameIndex adalah HMAC dari username
func usernameIndex(key [32]byte, username string) []byte {
	mac := hmac.New(sha256.New, key[:])
	mac.Write([]byte("users/username-index"))
	mac.Write([]byte(username))
	return mac.Sum(nil)
}
