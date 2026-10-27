package models

type File struct {
	ID     uint `gorm:"primaryKey"`
	UserID uint `gorm:"not null;index"`

	OriginalName string `gorm:"not null"`
	OriginalSize int64  `gorm:"not null"`
	OriginalType string `gorm:"not null"`

	// Version 0 is historical AES-only storage; version 1 requires all three ciphers.
	CipherSetVersion int `gorm:"not null;default:0"`

	AESPath string
	RC4Path string
	DESPath string

	AESEncryptTimeNS int64 `gorm:"column:aes_encrypt_time_ns;not null;default:0"`
	AESDecryptTimeNS int64 `gorm:"column:aes_decrypt_time_ns;not null;default:0"`
	DESEncryptTimeNS int64 `gorm:"column:des_encrypt_time_ns;not null;default:0"`
	DESDecryptTimeNS int64 `gorm:"column:des_decrypt_time_ns;not null;default:0"`
	RC4EncryptTimeNS int64 `gorm:"column:rc4_encrypt_time_ns;not null;default:0"`
	RC4DecryptTimeNS int64 `gorm:"column:rc4_decrypt_time_ns;not null;default:0"`
}
