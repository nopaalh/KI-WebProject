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

func (r *FileRepository) ListByUserID(userID uint) ([]models.File, error) {
	files := make([]models.File, 0)
	if err := r.DB.Where("user_id = ?", userID).Order("id DESC").Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}

func (r *FileRepository) FindByIDAndUserID(fileID, userID uint) (*models.File, error) {
	var file models.File
	if err := r.DB.Where("id = ? AND user_id = ?", fileID, userID).First(&file).Error; err != nil {
		return nil, err
	}
	return &file, nil
}

func (r *FileRepository) UpdateDecryptTime(fileID, userID uint, algorithm string, durationNS int64) error {
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

	result := r.DB.Model(&models.File{}).
		Where("id = ? AND user_id = ?", fileID, userID).
		Update(column, durationNS)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
