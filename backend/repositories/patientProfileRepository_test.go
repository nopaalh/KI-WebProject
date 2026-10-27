package repositories

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"project1/models"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newDryRunPatientProfileRepository(t *testing.T) *PatientProfileRepository {
	t.Helper()
	// Skip transactions as well as ping so DryRun never contacts a database.
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=127.0.0.1 port=1 user=unused dbname=unused sslmode=disable",
	}), &gorm.Config{
		DryRun:                 true,
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return NewPatientProfileRepository(db)
}

func TestPatientProfileFindByUserIDScopesQuery(t *testing.T) {
	repo := newDryRunPatientProfileRepository(t)
	var query string
	var args []any
	if err := repo.DB.Callback().Query().After("gorm:query").Register("capture_profile_query", func(db *gorm.DB) {
		query = db.Statement.SQL.String()
		args = append([]any(nil), db.Statement.Vars...)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindByUserID(7); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, `FROM "patient_profiles" WHERE user_id = $1`) || len(args) == 0 || args[0] != uint(7) {
		t.Fatalf("profile read must bind authenticated owner ID: %s, %v", query, args)
	}
}

func TestPatientProfileFindByUserIDPropagatesErrors(t *testing.T) {
	for _, expected := range []error{gorm.ErrRecordNotFound, errors.New("database unavailable")} {
		repo := newDryRunPatientProfileRepository(t)
		if err := repo.DB.Callback().Query().Before("gorm:query").Register("fail_profile_query", func(db *gorm.DB) {
			db.AddError(expected)
		}); err != nil {
			t.Fatal(err)
		}
		profile, err := repo.FindByUserID(7)
		if profile != nil || !errors.Is(err, expected) {
			t.Fatalf("expected query error without data: %v, %v", profile, err)
		}
	}
}

func TestPatientProfileCreateBindsEncryptedFieldsAndUserID(t *testing.T) {
	repo := newDryRunPatientProfileRepository(t)
	profile := &models.PatientProfile{
		UserID:        7,
		WeightKG:      65.5,
		HeightCM:      170.5,
		Age:           30,
		NIKAES:        []byte{0x00, 0xff, 0x01},
		NIKDES:        []byte{0x02, 0xfe, 0x00},
		NIKRC4:        []byte{0x05, 0xfb, 0x00},
		BPJSNumberAES: []byte{0x03, 0xfd, 0x00},
		BPJSNumberDES: []byte{0x00, 0xfc, 0x04},
		BPJSNumberRC4: []byte{0x00, 0xfa, 0x06},
		KTP:           &models.PatientKTP{},
	}
	var query string
	var args []any
	creates := 0
	if err := repo.DB.Callback().Create().After("gorm:create").Register("capture_patient_profile_insert", func(db *gorm.DB) {
		creates++
		query = db.Statement.SQL.String()
		args = append([]any(nil), db.Statement.Vars...)
	}); err != nil {
		t.Fatal(err)
	}

	if err := repo.Create(profile); err != nil {
		t.Fatal(err)
	}
	if creates != 1 {
		t.Fatalf("expected one create with no association inserts, got %d", creates)
	}
	wantQuery := `INSERT INTO "patient_profiles" ("user_id","user_role","weight_kg","height_cm","age","nik_aes","nik_des","nik_rc4","bpjs_number_aes","bpjs_number_des","bpjs_number_rc4") VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING "id"`
	if query != wantQuery {
		t.Fatalf("unexpected insert SQL: %s", query)
	}
	wantArgs := []any{
		uint(7), "patient", float64(65.5), float64(170.5), 30,
		profile.NIKAES, profile.NIKDES, profile.NIKRC4,
		profile.BPJSNumberAES, profile.BPJSNumberDES, profile.BPJSNumberRC4,
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("insert must bind authenticated user ID, unchanged AES/DES/RC4 ciphertext bytes: got %#v, want %#v", args, wantArgs)
	}
}

func TestPatientProfileCreateMapsDuplicateAndPropagatesErrors(t *testing.T) {
	genericError := errors.New("create failed")
	for _, tc := range []struct {
		name    string
		dbError error
		want    error
	}{
		{"duplicate", &pgconn.PgError{Code: "23505"}, ErrPatientProfileExists},
		{"wrapped duplicate", fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23505"}), ErrPatientProfileExists},
		{"other postgres error", &pgconn.PgError{Code: "08006"}, nil},
		{"generic error", genericError, nil},
		{"generic duplicate message", errors.New("23505: duplicate key"), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newDryRunPatientProfileRepository(t)
			if err := repo.DB.Callback().Create().Before("gorm:create").Register("fail_patient_profile_insert", func(db *gorm.DB) {
				db.AddError(tc.dbError)
			}); err != nil {
				t.Fatal(err)
			}
			want := tc.want
			if want == nil {
				want = tc.dbError
			}
			if err := repo.Create(&models.PatientProfile{UserID: 7}); err != want {
				t.Fatalf("got %v, want %v", err, want)
			}
		})
	}
}
