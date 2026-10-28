package services

import (
	"errors"
	"testing"

	"project1/repositories"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRegisterMapsDuplicateUsername(t *testing.T) {
	for _, tc := range []struct {
		name    string
		dbError error
		want    error
	}{
		{"duplicate", &pgconn.PgError{Code: "23505"}, ErrUsernameExist},
		{"other database failure", &pgconn.PgError{Code: "08006"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(postgres.New(postgres.Config{
				DSN: "host=127.0.0.1 port=1 user=unused dbname=unused sslmode=disable",
			}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true, Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { sqlDB.Close() })
			if err := db.Callback().Create().Before("gorm:create").Register("database_failure", func(db *gorm.DB) { db.AddError(tc.dbError) }); err != nil {
				t.Fatal(err)
			}
			service := NewAuthService(repositories.NewUserRepository(db), [32]byte{})
			err = service.Register("Test Patient", "test_patient", "synthetic-password")
			want := tc.want
			if want == nil {
				want = tc.dbError
			}
			if !errors.Is(err, want) {
				t.Fatalf("got %v, want %v", err, want)
			}
		})
	}
}
