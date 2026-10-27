package models

const (
	RolePatient = "patient"
	RoleDoctor  = "doctor"
)

type User struct {
	ID           uint   `gorm:"primaryKey"`
	Name         string `gorm:"not null"`
	Username     string `gorm:"unique"`
	PasswordHash string `gorm:"not null"`
	Role         string `gorm:"not null;default:patient;check:users_role_check,role IN ('patient','doctor')"`

	Files []File `gorm:"foreignKey:UserID"`
}
