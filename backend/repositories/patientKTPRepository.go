package repositories

import (
	"project1/models"

	"gorm.io/gorm"
)

type PatientKTPRepository struct {
	DB *gorm.DB
}

func NewPatientKTPRepository(db *gorm.DB) *PatientKTPRepository {
	return &PatientKTPRepository{DB: db}
}

func (r *PatientKTPRepository) ownedQuery(userID uint) *gorm.DB {
	return r.DB.Model(&models.PatientKTP{}).
		Joins("JOIN patient_profiles ON patient_profiles.id = patient_ktps.patient_profile_id").
		Where("patient_profiles.user_id = ?", userID)
}

func (r *PatientKTPRepository) FindByUserID(userID uint) (*models.PatientKTP, error) {
	var ktp models.PatientKTP
	// Reading identity fields must not load three potentially large photo blobs.
	if err := r.ownedQuery(userID).Omit("photo_aes", "photo_des", "photo_rc4").First(&ktp).Error; err != nil {
		return nil, err
	}
	return &ktp, nil
}

func (r *PatientKTPRepository) FindPhotoByUserID(userID uint) (*models.PatientKTP, error) {
	var ktp models.PatientKTP
	if err := r.ownedQuery(userID).Select(
		"patient_ktps.id", "patient_ktps.patient_profile_id",
		"patient_ktps.nik_aes", "patient_ktps.nik_des", "patient_ktps.nik_rc4",
		"patient_ktps.photo_aes", "patient_ktps.photo_des", "patient_ktps.photo_rc4",
	).First(&ktp).Error; err != nil {
		return nil, err
	}
	return &ktp, nil
}
