package repositories

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"project1/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newDryRunDoctorRepository(t *testing.T) *DoctorRepository {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=127.0.0.1 port=1 user=unused dbname=unused sslmode=disable",
	}), &gorm.Config{
		DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true,
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return NewDoctorRepository(db)
}

func TestDoctorListPatientsQueryAndEmptySlice(t *testing.T) {
	repo := newDryRunDoctorRepository(t)
	var query string
	var args []any
	queries := 0
	if err := repo.DB.Callback().Query().After("gorm:query").Register("capture_doctor_directory", func(db *gorm.DB) {
		queries++
		query = db.Statement.SQL.String()
		args = append([]any(nil), db.Statement.Vars...)
	}); err != nil {
		t.Fatal(err)
	}
	patients, err := repo.ListPatients()
	if err != nil {
		t.Fatal(err)
	}
	want := `SELECT profiles.id AS patient_profile_id, users.id AS user_id, users.name, users.username FROM patient_profiles AS profiles JOIN users ON users.id = profiles.user_id WHERE users.role = $1 AND profiles.user_role = $2 ORDER BY profiles.id ASC`
	if queries != 1 || query != want || !reflect.DeepEqual(args, []any{models.RolePatient, models.RolePatient}) {
		t.Fatalf("expected one bound, patient-only summary query: count=%d SQL=%s args=%v", queries, query, args)
	}
	encoded, err := json.Marshal(patients)
	if err != nil || string(encoded) != "[]" {
		t.Fatalf("expected empty array, got %s, %v", encoded, err)
	}
}

func TestDoctorListPatientsPropagatesDatabaseError(t *testing.T) {
	repo := newDryRunDoctorRepository(t)
	want := errors.New("database unavailable")
	if err := repo.DB.Callback().Query().Before("gorm:query").Register("fail_doctor_directory", func(db *gorm.DB) {
		db.AddError(want)
	}); err != nil {
		t.Fatal(err)
	}
	patients, err := repo.ListPatients()
	if patients != nil || !errors.Is(err, want) {
		t.Fatalf("expected database error without data: %v, %v", patients, err)
	}
}

func TestPatientSummaryJSON(t *testing.T) {
	encoded, err := json.Marshal(PatientSummary{PatientProfileID: 3, UserID: 7, Name: "Patient", Username: "patient"})
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"patient_profile_id":3,"user_id":7,"name":"Patient","username":"patient"}` {
		t.Fatalf("unexpected summary fields: %s", encoded)
	}
}
