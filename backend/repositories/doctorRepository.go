package repositories

import (
	"project1/models"

	"gorm.io/gorm"
)

// untuk jawaban JSON
type PatientSummary struct {
	PatientProfileID uint   `json:"patient_profile_id"`
	UserID           uint   `json:"user_id"`
	Name             string `json:"name"`
	Username         string `json:"username"`
}

// PatientRow: baris mentah dari database (nama dan username masih berupa amplop)
type PatientRow struct {
	PatientProfileID uint   `gorm:"column:patient_profile_id"`
	UserID           uint   `gorm:"column:user_id"`
	NameAES          []byte `gorm:"column:name_aes"`
	NameDES          []byte `gorm:"column:name_des"`
	NameRC4          []byte `gorm:"column:name_rc4"`
	UsernameAES      []byte `gorm:"column:username_aes"`
	UsernameDES      []byte `gorm:"column:username_des"`
	UsernameRC4      []byte `gorm:"column:username_rc4"`
}

type DoctorRepository struct {
	DB *gorm.DB
}

func NewDoctorRepository(db *gorm.DB) *DoctorRepository {
	return &DoctorRepository{DB: db}
}

func (r *DoctorRepository) ListPatients() ([]PatientRow, error) {
	patients := make([]PatientRow, 0)
	err := r.DB.Table("patient_profiles AS profiles").
		Select("profiles.id AS patient_profile_id, users.id AS user_id, "+
			"users.name_aes, users.name_des, users.name_rc4, "+
			"users.username_aes, users.username_des, users.username_rc4").
		Joins("JOIN users ON users.id = profiles.user_id").
		Where("users.role = ? AND profiles.user_role = ?", models.RolePatient, models.RolePatient).
		Order("profiles.id ASC").
		Find(&patients).Error
	if err != nil {
		return nil, err
	}
	return patients, nil
}
