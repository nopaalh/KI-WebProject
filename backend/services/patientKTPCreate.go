package services

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"strings"
	"time"
	"unicode/utf8"

	"project1/models"
	"project1/repositories"
	"project1/utils"
)

var (
	ErrInvalidKTP = errors.New("invalid patient KTP")
	ErrKTPExists  = repositories.ErrPatientKTPExists
)

type CreatePatientKTPInput struct {
	FullName      string `json:"full_name"`
	BirthPlace    string `json:"birth_place"`
	BirthDate     string `json:"birth_date"`
	Gender        string `json:"gender"`
	BloodType     string `json:"blood_type"`
	Address       string `json:"address"`
	RT            string `json:"rt"`
	RW            string `json:"rw"`
	Village       string `json:"village"`
	District      string `json:"district"`
	Religion      string `json:"religion"`
	MaritalStatus string `json:"marital_status"`
	Occupation    string `json:"occupation"`
	Citizenship   string `json:"citizenship"`
}

func (s *PatientKTPService) Create(userID uint, input CreatePatientKTPInput, photo []byte) (*models.PatientKTP, error) {
	profile, err := s.profiles.Get(userID)
	if err != nil {
		return nil, err
	}
	ktp := &models.PatientKTP{PatientProfileID: profile.ID}
	fields := []struct {
		name   string
		text   string
		target *models.KTPEncryptedValue
	}{
		{"nik", profile.NIK, &ktp.NIK}, {"full_name", input.FullName, &ktp.FullName},
		{"birth_place", input.BirthPlace, &ktp.BirthPlace}, {"birth_date", input.BirthDate, &ktp.BirthDate},
		{"gender", input.Gender, &ktp.Gender}, {"blood_type", input.BloodType, &ktp.BloodType},
		{"address", input.Address, &ktp.Address}, {"rt", input.RT, &ktp.RT}, {"rw", input.RW, &ktp.RW},
		{"village", input.Village, &ktp.Village}, {"district", input.District, &ktp.District},
		{"religion", input.Religion, &ktp.Religion}, {"marital_status", input.MaritalStatus, &ktp.MaritalStatus},
		{"occupation", input.Occupation, &ktp.Occupation}, {"citizenship", input.Citizenship, &ktp.Citizenship},
	}
	for _, field := range fields {
		if !utf8.ValidString(field.text) || strings.TrimSpace(field.text) == "" || len(field.text) > maxKTPTextSize {
			return nil, fmt.Errorf("%w: %s must be nonempty UTF-8 text of at most 16 KiB", ErrInvalidKTP, field.name)
		}
	}
	birthDate, err := time.Parse("2006-01-02", input.BirthDate)
	if err != nil || birthDate.After(time.Now().UTC()) {
		return nil, fmt.Errorf("%w: birth_date must be a valid YYYY-MM-DD date, not in the future", ErrInvalidKTP)
	}
	if input.Gender != "Laki-laki" && input.Gender != "Perempuan" {
		return nil, fmt.Errorf("%w: gender must be Laki-laki or Perempuan", ErrInvalidKTP)
	}
	switch input.BloodType {
	case "A", "B", "AB", "O", "-":
	default:
		return nil, fmt.Errorf("%w: blood_type must be A, B, AB, O, or -", ErrInvalidKTP)
	}
	if input.Citizenship != "WNI" && input.Citizenship != "WNA" {
		return nil, fmt.Errorf("%w: citizenship must be WNI or WNA", ErrInvalidKTP)
	}
	switch input.MaritalStatus {
	case "Belum Kawin", "Kawin", "Cerai Hidup", "Cerai Mati":
	default:
		return nil, fmt.Errorf("%w: invalid marital_status", ErrInvalidKTP)
	}
	for _, value := range []string{input.RT, input.RW} {
		if value != "-" && (len(value) < 1 || len(value) > 3 || !profileDigits(value, len(value))) {
			return nil, fmt.Errorf("%w: rt/rw must be 1-3 digits or -", ErrInvalidKTP)
		}
	}
	if len(photo) == 0 || len(photo) > MaxKTPPhotoSize {
		return nil, fmt.Errorf("%w: photo is required and must not exceed 8 MiB", ErrInvalidKTP)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(photo))
	if err != nil || (format != "png" && format != "jpeg") || config.Width <= 0 || config.Height <= 0 || config.Width > maxKTPPixels/config.Height {
		return nil, fmt.Errorf("%w: photo must be PNG/JPEG with at most 25 million pixels", ErrInvalidKTP)
	}
	// Check the full image only after bounding dimensions; preserve the original encoded bytes.
	if _, _, err := image.Decode(bytes.NewReader(photo)); err != nil {
		return nil, fmt.Errorf("%w: photo is incomplete or invalid", ErrInvalidKTP)
	}
	for _, field := range fields {
		*field.target, err = s.encryptValue([]byte(field.text), maxKTPTextSize)
		if err != nil {
			return nil, err
		}
	}
	ktp.Photo, err = s.encryptValue(photo, MaxKTPPhotoSize)
	if err != nil {
		return nil, err
	}
	if err := s.ktps.Create(ktp); err != nil {
		return nil, err
	}
	return ktp, nil
}

func (s *PatientKTPService) encryptValue(plaintext []byte, limit int) (models.KTPEncryptedValue, error) {
	var value models.KTPEncryptedValue
	var err error
	value.AES, err = utils.EncryptAESFile(plaintext, s.key)
	if err != nil {
		return models.KTPEncryptedValue{}, fmt.Errorf("encrypt KTP with AES: %w", err)
	}
	value.DES, err = utils.EncryptDESFile(plaintext, s.key)
	if err != nil {
		return models.KTPEncryptedValue{}, fmt.Errorf("encrypt KTP with DES: %w", err)
	}
	value.RC4, err = utils.EncryptRC4File(plaintext, s.key)
	if err != nil {
		return models.KTPEncryptedValue{}, fmt.Errorf("encrypt KTP with RC4: %w", err)
	}
	decoded, err := s.decryptValue(value, limit)
	if err != nil || !bytes.Equal(decoded, plaintext) {
		return models.KTPEncryptedValue{}, ErrKTPIntegrity
	}
	return value, nil
}
