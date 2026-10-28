package repositories

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"project1/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func newDryRunFileRepository(t *testing.T) *FileRepository {
	t.Helper()
	// DryRun, disabled ping, and disabled transactions prevent all DB contact.
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

func captureFileQuery(t *testing.T, repo *FileRepository) (*string, *[]any) {
	t.Helper()
	var query string
	var args []any
	if err := repo.DB.Callback().Query().After("gorm:query").Register("capture_query", func(db *gorm.DB) {
		query = db.Statement.SQL.String()
		args = append([]any(nil), db.Statement.Vars...)
	}); err != nil {
		t.Fatal(err)
	}
	return &query, &args
}

func accessibleSQL(role string, firstArg int) string {
	if role == models.RolePatient {
		return `(files.patient_profile_id IN (SELECT "id" FROM "patient_profiles" WHERE user_id = $` + strconv.Itoa(firstArg) + `) OR (files.patient_profile_id IS NULL AND files.user_id = $` + strconv.Itoa(firstArg+1) + `))`
	}
	return `(files.patient_profile_id IN (SELECT "id" FROM "patient_profiles") OR (files.patient_profile_id IS NULL AND files.user_id = $` + strconv.Itoa(firstArg) + `))`
}

func scopeArgs(role string) []any {
	if role == models.RolePatient {
		return []any{uint(7), uint(7)}
	}
	return []any{uint(7)}
}

func TestListAccessibleSubjectScopeAndFilter(t *testing.T) {
	for _, role := range []string{models.RolePatient, models.RoleDoctor} {
		for _, filtered := range []bool{false, true} {
			t.Run(role+map[bool]string{false: "/all", true: "/filtered"}[filtered], func(t *testing.T) {
				repo := newDryRunFileRepository(t)
				query, args := captureFileQuery(t, repo)
				var filter *uint
				profileID := uint(42)
				if filtered {
					filter = &profileID
				}
				files, err := repo.ListAccessible(7, role, filter)
				if err != nil || files == nil || len(files) != 0 {
					t.Fatalf("expected initialized empty result: %v, %v", files, err)
				}
				want := `SELECT * FROM "files" WHERE ` + accessibleSQL(role, 1)
				wantArgs := scopeArgs(role)
				if filtered {
					want = `SELECT * FROM "files" WHERE (` + accessibleSQL(role, 1) + ") AND files.patient_profile_id = $" + strconv.Itoa(len(wantArgs)+1)
					wantArgs = append(wantArgs, profileID)
				}
				want += " ORDER BY files.id DESC"
				if *query != want || !reflect.DeepEqual(*args, wantArgs) {
					t.Fatalf("subject/legacy scope mismatch:\nSQL: %s\nargs: %v\nwant: %s %v", *query, *args, want, wantArgs)
				}
			})
		}
	}
}

func TestFindAccessibleByIDScopesID(t *testing.T) {
	for _, role := range []string{models.RolePatient, models.RoleDoctor} {
		t.Run(role, func(t *testing.T) {
			repo := newDryRunFileRepository(t)
			query, args := captureFileQuery(t, repo)
			if _, err := repo.FindAccessibleByID(9, 7, role); err != nil {
				t.Fatal(err)
			}
			wantArgs := append(scopeArgs(role), uint(9), 1)
			want := `SELECT * FROM "files" WHERE (` + accessibleSQL(role, 1) + ") AND files.id = $" + strconv.Itoa(len(wantArgs)-1) + ` ORDER BY "files"."id" LIMIT $` + strconv.Itoa(len(wantArgs))
			if *query != want || !reflect.DeepEqual(*args, wantArgs) {
				t.Fatalf("unexpected scoped find: %s %v; want %s %v", *query, *args, want, wantArgs)
			}
		})
	}
}

func TestFileAccessRejectsInvalidActorBeforeDBAccess(t *testing.T) {
	for _, actor := range []struct {
		userID uint
		role   string
	}{
		{0, models.RolePatient}, {0, models.RoleDoctor}, {7, ""}, {7, "admin"},
		{7, "Patient"}, {7, `doctor' OR 1=1 --`},
	} {
		repo := NewFileRepository(nil)
		if files, err := repo.ListAccessible(actor.userID, actor.role, nil); err == nil || files != nil {
			t.Fatalf("list accepted invalid actor: %+v", actor)
		}
		if file, err := repo.FindAccessibleByID(9, actor.userID, actor.role); err == nil || file != nil {
			t.Fatalf("find accepted invalid actor: %+v", actor)
		}
		if err := repo.UpdateDecryptTime(9, actor.userID, actor.role, "aes", 123); err == nil {
			t.Fatalf("update accepted invalid actor: %+v", actor)
		}
	}
}

func TestUpdateDecryptTimeScopesWhitelistedUpdates(t *testing.T) {
	for _, role := range []string{models.RolePatient, models.RoleDoctor} {
		for _, algorithm := range []string{"aes", "des", "rc4"} {
			t.Run(role+"/"+algorithm, func(t *testing.T) {
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
					if err := repo.UpdateDecryptTime(9, 7, role, algorithm, durationNS); err != nil {
						t.Fatal(err)
					}
					wantArgs := append([]any{durationNS}, scopeArgs(role)...)
					wantArgs = append(wantArgs, uint(9))
					want := `UPDATE "files" SET "` + algorithm + `_decrypt_time_ns"=$1 WHERE (` + accessibleSQL(role, 2) + ") AND files.id = $" + strconv.Itoa(len(wantArgs))
					if query != want || !reflect.DeepEqual(args, wantArgs) {
						t.Fatalf("unexpected scoped update: %s %v; want %s %v", query, args, want, wantArgs)
					}
				}
			})
		}
	}
}

func TestUpdateDecryptTimeRejectsInvalidAlgorithmBeforeDBAccess(t *testing.T) {
	for _, algorithm := range []string{"", "AES", "unknown", `aes_decrypt_time_ns = 0; DROP TABLE files;--`} {
		repo := NewFileRepository(nil)
		if err := repo.UpdateDecryptTime(9, 7, models.RolePatient, algorithm, 123); err == nil {
			t.Fatalf("accepted unsupported algorithm %q", algorithm)
		}
	}
}

func TestUpdateDecryptTimeReturnsNotFound(t *testing.T) {
	for _, role := range []string{models.RolePatient, models.RoleDoctor} {
		for _, algorithm := range []string{"aes", "des", "rc4"} {
			repo := newDryRunFileRepository(t)
			if err := repo.UpdateDecryptTime(9, 7, role, algorithm, 123); !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Fatalf("expected not found for zero affected rows, got %v", err)
			}
		}
	}
}

func TestFileRepositoryPropagatesErrors(t *testing.T) {
	queryError := errors.New("query failed")
	for _, injected := range []error{queryError, gorm.ErrRecordNotFound} {
		repo := newDryRunFileRepository(t)
		if err := repo.DB.Callback().Query().Before("gorm:query").Register("fail_query", func(db *gorm.DB) {
			db.AddError(injected)
		}); err != nil {
			t.Fatal(err)
		}
		if files, err := repo.ListAccessible(7, models.RolePatient, nil); !errors.Is(err, injected) || files != nil {
			t.Fatalf("expected list error without partial results: %v, %v", files, err)
		}
		if file, err := repo.FindAccessibleByID(9, 7, models.RoleDoctor); !errors.Is(err, injected) || file != nil {
			t.Fatalf("expected find error without result: %v, %v", file, err)
		}
	}
	for _, algorithm := range []string{"aes", "des", "rc4"} {
		repo := newDryRunFileRepository(t)
		if err := repo.DB.Callback().Update().Before("gorm:update").Register("fail_update", func(db *gorm.DB) {
			db.AddError(queryError)
		}); err != nil {
			t.Fatal(err)
		}
		if err := repo.UpdateDecryptTime(9, 7, models.RolePatient, algorithm, 123); !errors.Is(err, queryError) {
			t.Fatalf("expected update error rather than not found: %v", err)
		}
	}
	repo := newDryRunFileRepository(t)
	if err := repo.DB.Callback().Create().Before("gorm:create").Register("fail_upload", func(db *gorm.DB) {
		db.AddError(queryError)
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UploadFile(&models.File{}); !errors.Is(err, queryError) {
		t.Fatalf("expected upload error: %v", err)
	}
}

func TestUploadFileIncludesSubjectPathsAndTimings(t *testing.T) {
	profileID := uint(42)
	for _, subject := range []*uint{nil, &profileID} {
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
			UserID: 7, PatientProfileID: subject,
			OriginalNameAES: []byte("aes-name"), OriginalNameDES: []byte("des-name"), OriginalNameRC4: []byte("rc4-name"), OriginalSize: 42, OriginalType: "image/png", CipherSetVersion: 1,
			AESPath: "aes/ktp", RC4Path: "rc4/ktp", DESPath: "des/ktp",
			AESEncryptTimeNS: 1, AESDecryptTimeNS: 2,
			DESEncryptTimeNS: 3, DESDecryptTimeNS: 4,
			RC4EncryptTimeNS: 5, RC4DecryptTimeNS: 6,
		}
		if err := repo.UploadFile(file); err != nil {
			t.Fatal(err)
		}
		for _, column := range []string{
			"user_id", "patient_profile_id", "cipher_set_version", "aes_path", "des_path", "rc4_path",
			"aes_encrypt_time_ns", "aes_decrypt_time_ns", "des_encrypt_time_ns", "des_decrypt_time_ns",
			"rc4_encrypt_time_ns", "rc4_decrypt_time_ns",
		} {
			if !strings.Contains(query, `"`+column+`"`) {
				t.Fatalf("insert missing column %s: %s", column, query)
			}
		}
		wantArgs := []any{uint(7), subject, []byte("aes-name"), []byte("des-name"), []byte("rc4-name"), int64(42), "image/png", 1, "aes/ktp", "rc4/ktp", "des/ktp",
			int64(1), int64(2), int64(3), int64(4), int64(5), int64(6)}
		if !reflect.DeepEqual(args, wantArgs) {
			t.Fatalf("unexpected insert bindings: %v", args)
		}
		if file.PatientProfileID != subject {
			t.Fatal("upload must not assign a subject to legacy files")
		}
	}
}

func TestFilePatientProfileAssociation(t *testing.T) {
	s, err := schema.Parse(&models.PatientProfile{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	relation := s.Relationships.Relations["Attachments"]
	if relation == nil || relation.Type != schema.HasMany || relation.Field.Tag.Get("json") != "-" {
		t.Fatal("expected hidden has-many attachments")
	}
	constraint := relation.ParseConstraint()
	if constraint == nil || constraint.OnDelete != "RESTRICT" || constraint.OnUpdate != "RESTRICT" || constraint.ReferenceSchema.Table != "patient_profiles" || len(constraint.ForeignKeys) != 1 || constraint.ForeignKeys[0].DBName != "patient_profile_id" || constraint.References[0].DBName != "id" {
		t.Fatalf("unexpected attachment FK: %+v", constraint)
	}
	field := relation.FieldSchema.LookUpField("PatientProfileID")
	if field.FieldType.Kind() != reflect.Pointer || field.NotNull || field.TagSettings["INDEX"] != "INDEX" {
		t.Fatalf("expected nullable indexed patient profile ID: %+v", field)
	}
}

func TestPatientProfileFindByID(t *testing.T) {
	repo := newDryRunFileRepository(t)
	query, args := captureFileQuery(t, repo)
	profiles := NewPatientProfileRepository(repo.DB)
	if _, err := profiles.FindByID(42); err != nil {
		t.Fatal(err)
	}
	if *query != `SELECT * FROM "patient_profiles" WHERE id = $1 ORDER BY "patient_profiles"."id" LIMIT $2` || !reflect.DeepEqual(*args, []any{uint(42), 1}) {
		t.Fatalf("unexpected profile lookup: %s %v", *query, *args)
	}
	if err := repo.DB.Callback().Query().Before("gorm:query").Register("missing_profile", func(db *gorm.DB) {
		db.AddError(gorm.ErrRecordNotFound)
	}); err != nil {
		t.Fatal(err)
	}
	if profile, err := profiles.FindByID(42); profile != nil || !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected missing profile: %v, %v", profile, err)
	}
}
