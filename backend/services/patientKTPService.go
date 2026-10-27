package services

import (
	"bytes"
	"errors"
	"fmt"

	"unicode/utf8"

	"project1/models"
	"project1/utils"

	"gorm.io/gorm"
)

const maxKTPTextSize = 16 << 10

var (
	ErrKTPNotFound  = errors.New("patient KTP not found")
	ErrKTPIntegrity = errors.New("patient KTP failed integrity verification")
)

type PatientKTPView struct {
	ID               uint   `json:"id"`
	PatientProfileID uint   `json:"patient_profile_id"`
	NIK              string `json:"nik"`
	FullName         string `json:"full_name"`
	BirthPlace       string `json:"birth_place"`
	BirthDate        string `json:"birth_date"`
	Gender           string `json:"gender"`
	BloodType        string `json:"blood_type"`
	Address          string `json:"address"`
	RT               string `json:"rt"`
	RW               string `json:"rw"`
	Village          string `json:"village"`
	District         string `json:"district"`
	Religion         string `json:"religion"`
	MaritalStatus    string `json:"marital_status"`
	Occupation       string `json:"occupation"`
	Citizenship      string `json:"citizenship"`
}

type patientKTPReader interface {
	FindByUserID(uint) (*models.PatientKTP, error)
}

type PatientKTPService struct {
	ktps     patientKTPReader
	profiles *PatientProfileService
	key      [32]byte
}

func NewPatientKTPService(ktps patientKTPReader, profiles *PatientProfileService, key [32]byte) *PatientKTPService {
	return &PatientKTPService{ktps: ktps, profiles: profiles, key: key}
}

func (s *PatientKTPService) loadOwned(userID uint) (*models.PatientKTP, *PatientProfileView, error) {
	profile, err := s.profiles.Get(userID)
	if err != nil {
		return nil, nil, err
	}
	ktp, err := s.ktps.FindByUserID(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, ErrKTPNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read patient KTP: %w", err)
	}
	if ktp == nil || ktp.PatientProfileID != profile.ID {
		return nil, nil, ErrKTPIntegrity
	}
	nik, err := s.decryptValue(ktp.NIK, maxKTPTextSize)
	if err != nil || string(nik) != profile.NIK {
		return nil, nil, ErrKTPIntegrity
	}
	return ktp, profile, nil
}

func (s *PatientKTPService) Get(userID uint) (*PatientKTPView, error) {
	ktp, profile, err := s.loadOwned(userID)
	if err != nil {
		return nil, err
	}
	view := &PatientKTPView{ID: ktp.ID, PatientProfileID: profile.ID, NIK: profile.NIK}
	for _, field := range []struct {
		value  models.KTPEncryptedValue
		target *string
	}{
		{ktp.FullName, &view.FullName}, {ktp.BirthPlace, &view.BirthPlace},
		{ktp.BirthDate, &view.BirthDate}, {ktp.Gender, &view.Gender},
		{ktp.BloodType, &view.BloodType}, {ktp.Address, &view.Address},
		{ktp.RT, &view.RT}, {ktp.RW, &view.RW}, {ktp.Village, &view.Village},
		{ktp.District, &view.District}, {ktp.Religion, &view.Religion},
		{ktp.MaritalStatus, &view.MaritalStatus}, {ktp.Occupation, &view.Occupation},
		{ktp.Citizenship, &view.Citizenship},
	} {
		plaintext, err := s.decryptValue(field.value, maxKTPTextSize)
		if err != nil || !utf8.Valid(plaintext) {
			return nil, ErrKTPIntegrity
		}
		*field.target = string(plaintext)
	}
	return view, nil
}

func (s *PatientKTPService) decryptValue(value models.KTPEncryptedValue, limit int) ([]byte, error) {
	var plaintext []byte
	for i, cipher := range []struct {
		data    []byte
		decrypt func([]byte, [32]byte) ([]byte, error)
	}{{value.AES, utils.DecryptAESFile}, {value.DES, utils.DecryptDESFile}, {value.RC4, utils.DecryptRC4File}} {
		// Bound envelope sizes before decryption allocates a plaintext buffer.
		if len(cipher.data) == 0 || len(cipher.data) > limit+128 {
			return nil, ErrKTPIntegrity
		}
		decoded, err := cipher.decrypt(cipher.data, s.key)
		if err != nil || len(decoded) > limit {
			return nil, ErrKTPIntegrity
		}
		if i == 0 {
			plaintext = decoded
		} else if !bytes.Equal(decoded, plaintext) {
			return nil, ErrKTPIntegrity
		}
	}
	return plaintext, nil
}
