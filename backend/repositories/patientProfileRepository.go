package repositories

import (
	"errors"

	"project1/models"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrPatientProfileExists = errors.New("patient profile already exists")

type PatientProfileRepository struct {
	DB *gorm.DB
}

func NewPatientProfileRepository(db *gorm.DB) *PatientProfileRepository {
	return &PatientProfileRepository{DB: db}
}

func (r *PatientProfileRepository) Create(profile *models.PatientProfile) error {
	err := r.DB.Omit(clause.Associations).Create(profile).Error
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return ErrPatientProfileExists
	}
	return err
}
