package services

import (
	"project1/models"
	"project1/repositories"
)

type DoctorService struct {
	users    currentUserReader
	patients interface {
		ListPatients() ([]repositories.PatientRow, error)
	}
	key [32]byte
}

func NewDoctorService(users currentUserReader, patients interface {
	ListPatients() ([]repositories.PatientRow, error)
}, key [32]byte) *DoctorService {
	return &DoctorService{users: users, patients: patients, key: key}
}

func (s *DoctorService) ListPatients(userID uint) ([]repositories.PatientSummary, error) {
	if _, err := authorizeUser(s.users, userID, models.RoleDoctor); err != nil {
		return nil, err
	}
	rows, err := s.patients.ListPatients()
	if err != nil {
		return nil, err
	}
	patients := make([]repositories.PatientSummary, 0, len(rows))
	for _, row := range rows {
		name, err := decryptTriple(s.key, row.NameAES, row.NameDES, row.NameRC4)
		if err != nil {
			return nil, err
		}
		username, err := decryptTriple(s.key, row.UsernameAES, row.UsernameDES, row.UsernameRC4)
		if err != nil {
			return nil, err
		}
		patients = append(patients, repositories.PatientSummary{
			PatientProfileID: row.PatientProfileID,
			UserID:           row.UserID,
			Name:             string(name),
			Username:         string(username),
		})
	}
	return patients, nil
}
