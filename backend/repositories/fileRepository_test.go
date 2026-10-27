package repositories

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"project1/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newDryRunFileRepository(t *testing.T) *FileRepository {
	t.Helper()
	// DryRun and disabled automatic ping ensure these tests never contact a DB.
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
	return NewFileRepository(db)
}

func TestUpdateDecryptTimeScopesWhitelistedUpdates(t *testing.T) {
	for _, algorithm := range []string{"aes", "des", "rc4"} {
		t.Run(algorithm, func(t *testing.T) {
			for _, durationNS := range []int64{123, 0} {
				repo := newDryRunFileRepository(t)
				var query string
				var args []any
				if err := repo.DB.Callback().Update().After("gorm:update").Register("capture_update", func(db *gorm.DB) {
					query = db.Statement.SQL.String()
					args = append([]any(nil), db.Statement.Vars...)
					db.RowsAffected = 1
				}); err != nil {
					t.Fatal(err)
				}

				if err := repo.UpdateDecryptTime(9, 7, algorithm, durationNS); err != nil {
					t.Fatal(err)
				}
				wantQuery := `UPDATE "files" SET "` + algorithm + `_decrypt_time_ns"=$1 WHERE id = $2 AND user_id = $3`
				if query != wantQuery {
					t.Fatalf("unexpected owner-scoped update SQL: %s", query)
				}
				if !reflect.DeepEqual(args, []any{durationNS, uint(9), uint(7)}) {
					t.Fatalf("unexpected update bindings: %v", args)
				}
			}
		})
	}
}

func TestUpdateDecryptTimeRejectsInvalidAlgorithmBeforeDBAccess(t *testing.T) {
	for _, algorithm := range []string{"", "AES", "unknown", `aes_decrypt_time_ns = 0; DROP TABLE files;--`} {
		t.Run(algorithm, func(t *testing.T) {
			// A nil DB ensures validation cannot access the database at all.
			repo := NewFileRepository(nil)
			if err := repo.UpdateDecryptTime(9, 7, algorithm, 123); err == nil {
				t.Fatal("expected unsupported algorithm error")
			}
		})
	}
}

func TestUpdateDecryptTimeReturnsNotFound(t *testing.T) {
	for _, algorithm := range []string{"aes", "des", "rc4"} {
		t.Run(algorithm, func(t *testing.T) {
			repo := newDryRunFileRepository(t)
			if err := repo.UpdateDecryptTime(9, 7, algorithm, 123); !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Fatalf("expected not found for zero affected rows, got %v", err)
			}
		})
	}
}

func TestUpdateDecryptTimePropagatesUpdateError(t *testing.T) {
	for _, algorithm := range []string{"aes", "des", "rc4"} {
		t.Run(algorithm, func(t *testing.T) {
			repo := newDryRunFileRepository(t)
			updateError := errors.New("update failed")
			if err := repo.DB.Callback().Update().Before("gorm:update").Register("fail_update", func(db *gorm.DB) {
				db.AddError(updateError)
			}); err != nil {
				t.Fatal(err)
			}
			if err := repo.UpdateDecryptTime(9, 7, algorithm, 123); !errors.Is(err, updateError) {
				t.Fatalf("expected update error rather than not found, got %v", err)
			}
		})
	}
}

func TestUploadFileIncludesPathsAndTimings(t *testing.T) {
	repo := newDryRunFileRepository(t)
	var query string
	var args []any
	if err := repo.DB.Callback().Create().After("gorm:create").Register("capture_upload", func(db *gorm.DB) {
		query = db.Statement.SQL.String()
		args = append([]any(nil), db.Statement.Vars...)
	}); err != nil {
		t.Fatal(err)
	}

	file := &models.File{
		UserID: 7, OriginalName: "ktp.png", OriginalSize: 42, OriginalType: "image/png", CipherSetVersion: 1,
		AESPath: "aes/ktp", RC4Path: "rc4/ktp", DESPath: "des/ktp",
		AESEncryptTimeNS: 1, AESDecryptTimeNS: 2,
		DESEncryptTimeNS: 3, DESDecryptTimeNS: 4,
		RC4EncryptTimeNS: 5, RC4DecryptTimeNS: 6,
	}
	if err := repo.UploadFile(file); err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{
		"cipher_set_version", "aes_path", "des_path", "rc4_path",
		"aes_encrypt_time_ns", "aes_decrypt_time_ns",
		"des_encrypt_time_ns", "des_decrypt_time_ns",
		"rc4_encrypt_time_ns", "rc4_decrypt_time_ns",
	} {
		if !strings.Contains(query, `"`+column+`"`) {
			t.Fatalf("insert missing column %s: %s", column, query)
		}
	}
	wantArgs := []any{uint(7), "ktp.png", int64(42), "image/png", 1, "aes/ktp", "rc4/ktp", "des/ktp",
		int64(1), int64(2), int64(3), int64(4), int64(5), int64(6)}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("unexpected insert bindings: %v", args)
	}
}

func TestListByUserIDScopesAndOrdersQuery(t *testing.T) {
	repo := newDryRunFileRepository(t)
	var query string
	var args []any
	if err := repo.DB.Callback().Query().After("gorm:query").Register("capture_list_query", func(db *gorm.DB) {
		query = db.Statement.SQL.String()
		args = append([]any(nil), db.Statement.Vars...)
	}); err != nil {
		t.Fatal(err)
	}

	files, err := repo.ListByUserID(7)
	if err != nil || files == nil || len(files) != 0 {
		t.Fatalf("expected empty initialized result in dry run: %v, %v", files, err)
	}
	if query != `SELECT * FROM "files" WHERE user_id = $1 ORDER BY id DESC` {
		t.Fatalf("unexpected owner-scoped ordered SQL: %s", query)
	}
	if len(args) != 1 || args[0] != uint(7) {
		t.Fatalf("query must bind the authenticated owner's ID: %v", args)
	}
}

func TestListByUserIDPropagatesQueryError(t *testing.T) {
	repo := newDryRunFileRepository(t)
	queryError := errors.New("query failed")
	if err := repo.DB.Callback().Query().Before("gorm:query").Register("fail_list_query", func(db *gorm.DB) {
		db.AddError(queryError)
	}); err != nil {
		t.Fatal(err)
	}
	files, err := repo.ListByUserID(7)
	if !errors.Is(err, queryError) || files != nil {
		t.Fatalf("expected query error without partial results: %v, %v", files, err)
	}
}
