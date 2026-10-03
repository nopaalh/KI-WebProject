package models

type File struct {
	ID     uint `gorm:"primaryKey"`
	UserID uint `gorm:"not null"`

	OriginalName string `gorm:"not null"`
	OriginalSize int64  `gorm:"not null"`
	OriginalType string `gorm:"not null"`

	AESPath string
	RC4Path string
	DESPath string
}
