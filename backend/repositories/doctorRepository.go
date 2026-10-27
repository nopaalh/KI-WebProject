package repositories

import (
	"project1/models"

	"gorm.io/gorm"
)

type PatientSummary struct {
	PatientProfileID uint   `json:"patient_profile_id"`
	UserID           uint   `json:"user_id"`
	Name             string `json:"name"`
	Username         string `json:"username"`
}

type DoctorRepository struct {
	DB *gorm.DB
}

func NewDoctorRepository(db *gorm.DB) *DoctorRepository {
	return &DoctorRepository{DB: db}
}

func (r *DoctorRepository) ListPatients() ([]PatientSummary, error) {
	patients := make([]PatientSummary, 0)
	err := r.DB.Table("patient_profiles AS profiles").
		Select("profiles.id AS patient_profile_id, users.id AS user_id, users.name, users.username").
		Joins("JOIN users ON users.id = profiles.user_id").
		Where("users.role = ? AND profiles.user_role = ?", models.RolePatient, models.RolePatient).
		Order("profiles.id ASC").
		Find(&patients).Error
	if err != nil {
		return nil, err
	}
	return patients, nil
}
