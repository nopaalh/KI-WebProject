package services

import (
	"bytes"
	"errors"
	"fmt"
	"math"

	"project1/models"
	"project1/repositories"
	"project1/utils"

	"gorm.io/gorm"
)

var (
	ErrProfileUnauthorized = errors.New("user is not authenticated or no longer exists")
	ErrProfileForbidden    = errors.New("only patients can access their own profile")
	ErrInvalidProfile      = errors.New("invalid patient profile")
	ErrProfileExists       = repositories.ErrPatientProfileExists
	ErrProfileNotFound     = errors.New("patient profile not found")
	ErrProfileIntegrity    = errors.New("patient profile failed integrity verification")
)

type CreatePatientProfileInput struct {
	WeightKG   float64 `json:"weight_kg"`
	HeightCM   float64 `json:"height_cm"`
	Age        *int    `json:"age"`
	NIK        string  `json:"nik"`
	BPJSNumber string  `json:"bpjs_number"`
}

type PatientProfileView struct {
	ID         uint    `json:"id"`
	UserID     uint    `json:"user_id"`
	WeightKG   float64 `json:"weight_kg"`
	HeightCM   float64 `json:"height_cm"`
	Age        int     `json:"age"`
	NIK        string  `json:"nik"`
	BPJSNumber string  `json:"bpjs_number"`
}

type patientProfileRepository interface {
	Create(*models.PatientProfile) error
	FindByUserID(uint) (*models.PatientProfile, error)
}

type profileUserReader interface {
	FindByID(uint) (*models.User, error)
}

type PatientProfileService struct {
	profiles patientProfileRepository
	users    profileUserReader
	key      [32]byte
}

func NewPatientProfileService(profiles patientProfileRepository, users profileUserReader, key [32]byte) *PatientProfileService {
	return &PatientProfileService{profiles: profiles, users: users, key: key}
}

func (s *PatientProfileService) authorizePatient(userID uint) error {
	if userID == 0 {
		return ErrProfileUnauthorized
	}
	user, err := s.users.FindByID(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrProfileUnauthorized
	}
	if err != nil {
		return fmt.Errorf("load profile owner: %w", err)
	}
	if user == nil || user.Role != models.RolePatient {
		return ErrProfileForbidden
	}
	return nil
}

func (s *PatientProfileService) Create(userID uint, input CreatePatientProfileInput) (*models.PatientProfile, error) {
	if err := s.authorizePatient(userID); err != nil {
		return nil, err
	}
	if err := validatePatientProfile(input); err != nil {
		return nil, err
	}

	profile := &models.PatientProfile{
		UserID: userID, UserRole: models.RolePatient,
		WeightKG: input.WeightKG, HeightCM: input.HeightCM, Age: *input.Age,
	}
	var err error
	profile.NIKAES, profile.NIKDES, profile.NIKRC4, err = s.encryptIdentity(input.NIK)
	if err != nil {
		return nil, err
	}
	profile.BPJSNumberAES, profile.BPJSNumberDES, profile.BPJSNumberRC4, err = s.encryptIdentity(input.BPJSNumber)
	if err != nil {
		return nil, err
	}

	if err := s.profiles.Create(profile); err != nil {
		return nil, err
	}
	return profile, nil
}

func (s *PatientProfileService) Get(userID uint) (*PatientProfileView, error) {
	if err := s.authorizePatient(userID); err != nil {
		return nil, err
	}
	profile, err := s.profiles.FindByUserID(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProfileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read patient profile: %w", err)
	}
	if profile == nil || profile.UserID != userID || profile.UserRole != models.RolePatient {
		return nil, ErrProfileIntegrity
	}
	// Only records with both RC4 columns NULL are considered legacy AES/DES profiles.
	if (profile.NIKRC4 == nil) != (profile.BPJSNumberRC4 == nil) {
		return nil, ErrProfileIntegrity
	}
	nik, err := s.decryptIdentity(profile.NIKAES, profile.NIKDES, profile.NIKRC4)
	if err != nil {
		return nil, err
	}
	bpjs, err := s.decryptIdentity(profile.BPJSNumberAES, profile.BPJSNumberDES, profile.BPJSNumberRC4)
	if err != nil {
		return nil, err
	}
	if !profileDigits(nik, 16) || !profileDigits(bpjs, 13) {
		return nil, ErrProfileIntegrity
	}
	return &PatientProfileView{
		ID: profile.ID, UserID: profile.UserID,
		WeightKG: profile.WeightKG, HeightCM: profile.HeightCM, Age: profile.Age,
		NIK: nik, BPJSNumber: bpjs,
	}, nil
}

func (s *PatientProfileService) decryptIdentity(aes, des, rc4 []byte) (string, error) {
	plaintext, err := utils.DecryptAESFile(aes, s.key)
	if err != nil {
		return "", ErrProfileIntegrity
	}
	decodedDES, err := utils.DecryptDESFile(des, s.key)
	if err != nil || !bytes.Equal(decodedDES, plaintext) {
		return "", ErrProfileIntegrity
	}
	if rc4 != nil {
		decodedRC4, err := utils.DecryptRC4File(rc4, s.key)
		if err != nil || !bytes.Equal(decodedRC4, plaintext) {
			return "", ErrProfileIntegrity
		}
	}
	return string(plaintext), nil
}

func validatePatientProfile(input CreatePatientProfileInput) error {
	for _, measurement := range []struct {
		name  string
		value float64
	}{{"weight_kg", input.WeightKG}, {"height_cm", input.HeightCM}} {
		v := measurement.value
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > 9999.99 || math.Abs(v*100-math.Round(v*100)) > 0.000001 {
			return fmt.Errorf("%w: %s must be positive, at most 9999.99, with up to two decimal places", ErrInvalidProfile, measurement.name)
		}
	}
	if input.Age == nil || *input.Age < 0 || *input.Age > 150 {
		return fmt.Errorf("%w: age is required and must be between 0 and 150", ErrInvalidProfile)
	}
	if !profileDigits(input.NIK, 16) {
		return fmt.Errorf("%w: nik must contain exactly 16 digits", ErrInvalidProfile)
	}
	if !profileDigits(input.BPJSNumber, 13) {
		return fmt.Errorf("%w: bpjs_number must contain exactly 13 digits", ErrInvalidProfile)
	}
	return nil
}

func profileDigits(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for i := range value {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func (s *PatientProfileService) encryptIdentity(value string) ([]byte, []byte, []byte, error) {
	plaintext := []byte(value)
	aes, err := utils.EncryptAESFile(plaintext, s.key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encrypt profile with AES: %w", err)
	}
	des, err := utils.EncryptDESFile(plaintext, s.key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encrypt profile with DES: %w", err)
	}
	rc4, err := utils.EncryptRC4File(plaintext, s.key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("encrypt profile with RC4: %w", err)
	}
	for _, cipher := range []struct {
		data    []byte
		decrypt func([]byte, [32]byte) ([]byte, error)
	}{{aes, utils.DecryptAESFile}, {des, utils.DecryptDESFile}, {rc4, utils.DecryptRC4File}} {
		decoded, err := cipher.decrypt(cipher.data, s.key)
		if err != nil || !bytes.Equal(decoded, plaintext) {
			return nil, nil, nil, errors.New("profile encryption round-trip verification failed")
		}
	}
	return aes, des, rc4, nil
}
