package models

const (
	RolePatient = "patient"
	RoleDoctor  = "doctor"
)

type User struct {
	ID           uint   `gorm:"primaryKey;uniqueIndex:idx_users_id_role,priority:1"`
	Name         string `gorm:"not null"`
	Username     string `gorm:"unique"`
	PasswordHash string `gorm:"not null"`
	Role         string `gorm:"not null;default:patient;check:users_role_check,role IN ('patient','doctor');uniqueIndex:idx_users_id_role,priority:2"`

	Files          []File          `gorm:"foreignKey:UserID"`
	PatientProfile *PatientProfile `gorm:"foreignKey:UserID,UserRole;references:ID,Role;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT" json:"-"`
}
