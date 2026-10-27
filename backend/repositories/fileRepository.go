package repositories

import (
	"fmt"
	"project1/models"

	"gorm.io/gorm"
)

type FileRepository struct {
	DB *gorm.DB
}

func NewFileRepository(db *gorm.DB) *FileRepository {
	return &FileRepository{DB: db}
}

func (r *FileRepository) UploadFile(file *models.File) error {
	return r.DB.Create(file).Error
}

func (r *FileRepository) accessibleQuery(userID uint, role string) (*gorm.DB, error) {
	if userID == 0 {
		return nil, fmt.Errorf("invalid user ID")
	}
	if role != models.RolePatient && role != models.RoleDoctor {
		return nil, fmt.Errorf("unsupported role: %q", role)
	}

	profiles := r.DB.Model(&models.PatientProfile{}).Select("id")
	if role == models.RolePatient {
		profiles = profiles.Where("user_id = ?", userID)
	}
	return r.DB.Model(&models.File{}).Where(
		"(files.patient_profile_id IN (?) OR (files.patient_profile_id IS NULL AND files.user_id = ?))",
		profiles, userID,
	), nil
}

func (r *FileRepository) ListAccessible(userID uint, role string, patientProfileID *uint) ([]models.File, error) {
	query, err := r.accessibleQuery(userID, role)
	if err != nil {
		return nil, err
	}
	if patientProfileID != nil {
		query = query.Where("files.patient_profile_id = ?", *patientProfileID)
	}
	files := make([]models.File, 0)
	if err := query.Order("files.id DESC").Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}

func (r *FileRepository) FindAccessibleByID(fileID, userID uint, role string) (*models.File, error) {
	query, err := r.accessibleQuery(userID, role)
	if err != nil {
		return nil, err
	}
	var file models.File
	if err := query.Where("files.id = ?", fileID).First(&file).Error; err != nil {
		return nil, err
	}
	return &file, nil
}

func (r *FileRepository) UpdateDecryptTime(fileID, userID uint, role, algorithm string, durationNS int64) error {
	var column string
	switch algorithm {
	case "aes":
		column = "aes_decrypt_time_ns"
	case "des":
		column = "des_decrypt_time_ns"
	case "rc4":
		column = "rc4_decrypt_time_ns"
	default:
		return fmt.Errorf("unsupported algorithm: %q", algorithm)
	}

	query, err := r.accessibleQuery(userID, role)
	if err != nil {
		return err
	}
	result := query.Where("files.id = ?", fileID).Update(column, durationNS)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
