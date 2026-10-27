package services

import (
	"project1/models"
	"project1/repositories"
)

type DoctorService struct {
	users    currentUserReader
	patients interface {
		ListPatients() ([]repositories.PatientSummary, error)
	}
}

func NewDoctorService(users currentUserReader, patients interface {
	ListPatients() ([]repositories.PatientSummary, error)
}) *DoctorService {
	return &DoctorService{users: users, patients: patients}
}

func (s *DoctorService) ListPatients(userID uint) ([]repositories.PatientSummary, error) {
	if _, err := authorizeUser(s.users, userID, models.RoleDoctor); err != nil {
		return nil, err
	}
	return s.patients.ListPatients()
}
