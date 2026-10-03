package models

type User struct {
	ID           uint   `gorm:"primaryKey"`
	Name         string `gorm:"not null"`
	Username     string `gorm:"unique"`
	PasswordHash string `gorm:"not null"`

	Files []File `gorm:"foreignKey:UserID"`
}
