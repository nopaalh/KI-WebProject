package services

import (
	"errors"
	"testing"

	"project1/models"

	"gorm.io/gorm"
)

type fileTestUsers struct {
	roles map[uint]string
	err   error
	calls int
}

func (r *fileTestUsers) FindByID(id uint) (*models.User, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	role := models.RolePatient
	if id == 50 {
		role = models.RoleDoctor
	}
	if value, ok := r.roles[id]; ok {
		role = value
	}
	return &models.User{ID: id, Role: role}, nil
}

type fileTestPatients struct {
	profileID uint
	missing   bool
	err       error
}

func (r *fileTestPatients) FindByID(id uint) (*models.PatientProfile, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.missing {
		return nil, gorm.ErrRecordNotFound
	}
	return &models.PatientProfile{ID: id, UserID: id, UserRole: models.RolePatient}, nil
}

func (r *fileTestPatients) FindByUserID(id uint) (*models.PatientProfile, error) {
	profile, err := r.FindByID(id)
	if profile != nil && r.profileID != 0 {
		profile.ID = r.profileID
	}
	return profile, err
}

func TestFileUploadKeepsProfileIDSeparateFromUserID(t *testing.T) {
	s, _, _ := newTestFileService(t)
	s.patients.(*fileTestPatients).profileID = 12
	file, err := s.Upload(7, uploadHeader(t, "own.pdf", []byte("own data")))
	if err != nil || file.UserID != 7 || file.PatientProfileID == nil || *file.PatientProfileID != 12 {
		t.Fatalf("patient profile ID must not be assumed equal to user ID: %v", err)
	}
}

func fileTestVisible(file models.File, actorID uint, role string, patientID *uint) bool {
	if role != models.RolePatient && role != models.RoleDoctor {
		return false
	}
	if patientID != nil && (file.PatientProfileID == nil || *file.PatientProfileID != *patientID) {
		return false
	}
	if file.PatientProfileID == nil {
		return file.UserID == actorID
	}
	return role == models.RoleDoctor || *file.PatientProfileID == actorID
}

func TestFileSubjectIsIndependentOfUploader(t *testing.T) {
	s, repo, _ := newTestFileService(t)
	file, err := s.UploadForPatient(50, 7, uploadHeader(t, "doctor-report.pdf", []byte("synthetic patient 7 report")))
	if err != nil {
		t.Fatal(err)
	}
	if file.UserID != 50 || file.PatientProfileID == nil || *file.PatientProfileID != 7 {
		t.Fatal("must retain doctor uploader separately from patient subject")
	}
	for _, reader := range []uint{7, 50, 51} {
		if reader == 51 {
			s.users.(*fileTestUsers).roles[51] = models.RoleDoctor
		}
		for _, cipher := range []string{"aes", "des", "rc4"} {
			if _, data, err := s.Decrypt(reader, file.ID, cipher); err != nil || string(data) != "synthetic patient 7 report" {
				t.Fatalf("authorized reader %d failed: %v", reader, err)
			}
		}
	}
	if _, _, err := s.Decrypt(8, file.ID, "aes"); !errors.Is(err, ErrFileNotFound) {
		t.Fatalf("another patient must not read the report: %v", err)
	}
	if files, err := s.List(7); err != nil || len(files) != 1 {
		t.Fatalf("subject must see doctor's report: %v", err)
	}
	if files, err := s.List(8); err != nil || len(files) != 0 {
		t.Fatalf("another patient must not list doctor's report: %v", err)
	}
	if repo.files[file.ID].UserID != 50 {
		t.Fatal("download timing must not reassign uploader")
	}
}

func TestFileRolesAndPatientTargetsFailClosed(t *testing.T) {
	s, repo, dir := newTestFileService(t)
	file := uploadHeader(t, "test.pdf", []byte("synthetic report"))
	if _, err := s.UploadForPatient(7, 8, file); !errors.Is(err, ErrAccessForbidden) {
		t.Fatalf("patient cannot upload for another patient: %v", err)
	}
	if _, err := s.Upload(50, file); !errors.Is(err, ErrAccessForbidden) {
		t.Fatalf("doctor must explicitly identify patient target: %v", err)
	}
	if _, err := s.UploadForPatient(50, 0, file); !errors.Is(err, ErrInvalidPatientID) {
		t.Fatalf("invalid target must be rejected: %v", err)
	}
	s.patients.(*fileTestPatients).missing = true
	if _, err := s.UploadForPatient(50, 8, file); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("missing target must not create artifacts: %v", err)
	}
	if _, err := s.Upload(7, file); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("patient needs own profile before uploading: %v", err)
	}
	assertNoFileUploadArtifacts(t, repo, dir)
}

func TestFileAccessUsesFreshDatabaseRoleOnce(t *testing.T) {
	s, _, _ := newTestFileService(t)
	file, err := s.UploadForPatient(50, 7, uploadHeader(t, "report.pdf", []byte("report")))
	if err != nil {
		t.Fatal(err)
	}
	users := s.users.(*fileTestUsers)
	users.calls = 0
	if _, _, err := s.Decrypt(50, file.ID, "des"); err != nil || users.calls != 1 {
		t.Fatalf("one role lookup per download expected: %v, calls=%d", err, users.calls)
	}
	users.roles[50] = models.RolePatient
	if _, _, err := s.Decrypt(50, file.ID, "aes"); !errors.Is(err, ErrFileNotFound) {
		t.Fatalf("former doctor uploader is not the patient subject: %v", err)
	}
	users.roles[50] = "admin"
	if _, err := s.List(50); !errors.Is(err, ErrAccessForbidden) {
		t.Fatalf("unknown roles must fail closed: %v", err)
	}
	users.err = gorm.ErrRecordNotFound
	if _, _, err := s.Decrypt(7, file.ID, "aes"); !errors.Is(err, ErrInvalidUser) {
		t.Fatalf("deleted account must be denied: %v", err)
	}
}
