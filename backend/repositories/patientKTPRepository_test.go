package repositories

import (
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"
)

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
