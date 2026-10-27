package repositories

import (
	"errors"
	"strings"
	"testing"

	"project1/models"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

func TestPatientKTPCreateBindsEveryCiphertext(t *testing.T) {
	db := newDryRunPatientProfileRepository(t).DB
	repo := NewPatientKTPRepository(db)
	value := models.KTPEncryptedValue{AES: []byte{1, 0, 255}, DES: []byte{2, 0, 254}, RC4: []byte{3, 0, 253}}
	ktp := &models.PatientKTP{
		PatientProfileID: 12,
		NIK:              value, FullName: value, BirthPlace: value, BirthDate: value, Gender: value, BloodType: value,
		Address: value, RT: value, RW: value, Village: value, District: value, Religion: value,
		MaritalStatus: value, Occupation: value, Citizenship: value, Photo: value,
	}
	var query string
	var args []any
	if err := db.Callback().Create().After("gorm:create").Register("capture_ktp_create", func(db *gorm.DB) {
		query = db.Statement.SQL.String()
		args = append([]any(nil), db.Statement.Vars...)
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ktp); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(query, `INSERT INTO "patient_ktps"`) || len(args) != 49 || args[0] != uint(12) {
		t.Fatalf("expected one insert with owner FK and 48 ciphertexts: %s, args=%d", query, len(args))
	}
	for _, name := range []string{"nik_aes", "nik_des", "nik_rc4", "photo_aes", "photo_des", "photo_rc4"} {
		if !strings.Contains(query, name) {
			t.Fatalf("missing ciphertext column %s", name)
		}
	}
	for i, arg := range args[1:] {
		if data, ok := arg.([]byte); !ok || len(data) == 0 {
			t.Fatalf("ciphertext %d must be bound as nonempty bytes", i)
		}
	}
}

func TestPatientKTPCreateMapsDuplicateAndPropagatesErrors(t *testing.T) {
	for _, tc := range []struct{ input, expected error }{
		{&pgconn.PgError{Code: "23505"}, ErrPatientKTPExists},
		{&pgconn.PgError{Code: "23503"}, nil},
		{errors.New("insert failed"), nil},
	} {
		db := newDryRunPatientProfileRepository(t).DB
		repo := NewPatientKTPRepository(db)
		if err := db.Callback().Create().Before("gorm:create").Register("fail_ktp_insert", func(db *gorm.DB) {
			db.AddError(tc.input)
		}); err != nil {
			t.Fatal(err)
		}
		expected := tc.expected
		if expected == nil {
			expected = tc.input
		}
		if err := repo.Create(&models.PatientKTP{}); !errors.Is(err, expected) {
			t.Fatalf("want %v, got %v", expected, err)
		}
	}
}

func TestPatientKTPQueryScopesOwnerAndOmitsPhoto(t *testing.T) {
	db := newDryRunPatientProfileRepository(t).DB
	repo := NewPatientKTPRepository(db)
	var query string
	var args []any
	if err := db.Callback().Query().After("gorm:query").Register("capture_ktp_read", func(db *gorm.DB) {
		query = db.Statement.SQL.String()
		args = append([]any(nil), db.Statement.Vars...)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindByUserID(7); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, "JOIN patient_profiles ON patient_profiles.id = patient_ktps.patient_profile_id") ||
		!strings.Contains(query, "patient_profiles.user_id = $1") || len(args) == 0 || args[0] != uint(7) {
		t.Fatalf("query must bind authenticated owner through profile join: %s, %v", query, args)
	}
	for _, column := range []string{"photo_aes", "photo_des", "photo_rc4"} {
		if strings.Contains(query, column) {
			t.Fatalf("text endpoint must not load photo blobs: %s", query)
		}
	}
	if !strings.Contains(query, "full_name_aes") {
		t.Fatalf("text endpoint must load identity fields: %s", query)
	}
}

func TestPatientKTPPhotoQueryScopesOwnerAndSelectsOnlyNeededFields(t *testing.T) {
	db := newDryRunPatientProfileRepository(t).DB
	repo := NewPatientKTPRepository(db)
	var query string
	var args []any
	if err := db.Callback().Query().After("gorm:query").Register("capture_ktp_photo", func(db *gorm.DB) {
		query = db.Statement.SQL.String()
		args = append([]any(nil), db.Statement.Vars...)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindPhotoByUserID(7); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, "JOIN patient_profiles ON patient_profiles.id = patient_ktps.patient_profile_id") ||
		!strings.Contains(query, "patient_profiles.user_id = $1") || len(args) == 0 || args[0] != uint(7) {
		t.Fatalf("photo query must bind authenticated owner: %s, %v", query, args)
	}
	for _, name := range []string{"photo_aes", "photo_des", "photo_rc4", "nik_aes", "nik_des", "nik_rc4"} {
		if !strings.Contains(query, name) {
			t.Fatalf("missing photo/NIK column %s: %s", name, query)
		}
	}
	if strings.Contains(query, "full_name_aes") || strings.Contains(query, "address_aes") {
		t.Fatalf("photo query should not fetch unrelated identity fields: %s", query)
	}
}

func TestPatientKTPPhotoQueryPropagatesErrors(t *testing.T) {
	for _, expected := range []error{gorm.ErrRecordNotFound, errors.New("database unavailable")} {
		db := newDryRunPatientProfileRepository(t).DB
		repo := NewPatientKTPRepository(db)
		if err := db.Callback().Query().Before("gorm:query").Register("fail_photo_query", func(db *gorm.DB) {
			db.AddError(expected)
		}); err != nil {
			t.Fatal(err)
		}
		photo, err := repo.FindPhotoByUserID(7)
		if photo != nil || !errors.Is(err, expected) {
			t.Fatalf("expected query error without photo: %v, %v", photo, err)
		}
	}
}

func TestPatientKTPQueryPropagatesErrors(t *testing.T) {
	for _, expected := range []error{gorm.ErrRecordNotFound, errors.New("database unavailable")} {
		db := newDryRunPatientProfileRepository(t).DB
		repo := NewPatientKTPRepository(db)
		if err := db.Callback().Query().Before("gorm:query").Register("fail_ktp_read", func(db *gorm.DB) {
			db.AddError(expected)
		}); err != nil {
			t.Fatal(err)
		}
		ktp, err := repo.FindByUserID(7)
		if ktp != nil || !errors.Is(err, expected) {
			t.Fatalf("expected query error without data: %v, %v", ktp, err)
		}
	}
}
