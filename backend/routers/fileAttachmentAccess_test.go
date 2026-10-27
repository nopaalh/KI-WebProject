package routers

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"project1/models"
	"project1/repositories"

	"gorm.io/gorm"
)

type fileRouteUsers struct {
	roles map[uint]string
	err   error
	calls int
}

func (r *fileRouteUsers) FindByID(id uint) (*models.User, error) {
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

type fileRoutePatients struct {
	missing bool
	err     error
}

func (r *fileRoutePatients) FindByID(id uint) (*models.PatientProfile, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.missing || id == 50 {
		return nil, gorm.ErrRecordNotFound
	}
	return &models.PatientProfile{ID: id, UserID: id, UserRole: models.RolePatient}, nil
}

func (r *fileRoutePatients) FindByUserID(id uint) (*models.PatientProfile, error) {
	return r.FindByID(id)
}

type fileRouteDirectory struct{}

func (*fileRouteDirectory) ListPatients() ([]repositories.PatientSummary, error) {
	return []repositories.PatientSummary{
		{PatientProfileID: 7, UserID: 7, Name: "Patient Seven", Username: "patient7"},
		{PatientProfileID: 8, UserID: 8, Name: "Patient Eight", Username: "patient8"},
	}, nil
}

func fileRouteVisible(file models.File, userID uint, role string, filter *uint) bool {
	if role != models.RolePatient && role != models.RoleDoctor {
		return false
	}
	if filter != nil && (file.PatientProfileID == nil || *file.PatientProfileID != *filter) {
		return false
	}
	if file.PatientProfileID == nil {
		return file.UserID == userID
	}
	return role == models.RoleDoctor || *file.PatientProfileID == userID
}

func uploadDoctorAttachment(t *testing.T, f *fileRouteFixture, patientID uint, token string, content []byte) fileRouteMetadata {
	t.Helper()
	body, contentType := fileRouteMultipart(t, "synthetic-report.pdf", content, 0)
	path := fmt.Sprintf("/api/doctor/patients/%d/files", patientID)
	response := fileRouteRequest(f.router, http.MethodPost, path, token, contentType, body)
	return decodeFileRouteResponse(t, response, 201).File
}

func TestDoctorReportBelongsToPatientNotUploader(t *testing.T) {
	f := newFileRouteFixture(t)
	doctor := fileRouteToken(t, 50)
	content := []byte("synthetic report for patient seven")
	file := uploadDoctorAttachment(t, f, 7, doctor, content)
	if file.UploadedByUserID != 50 || file.PatientProfileID == nil || *file.PatientProfileID != 7 || file.CipherSetVersion != 1 {
		t.Fatalf("uploader and patient subject must be separate: %+v", file)
	}
	for _, algorithm := range []string{"aes", "des", "rc4"} {
		path := fmt.Sprintf("/api/files/%d/decrypt?algorithm=%s", file.ID, algorithm)
		f.users.calls = 0
		response := fileRouteRequest(f.router, http.MethodPost, path, f.token, "", nil)
		if response.Code != 200 || !bytes.Equal(response.Body.Bytes(), content) || f.users.calls != 1 {
			t.Fatalf("patient must retrieve doctor's %s report with one role lookup: %d", algorithm, response.Code)
		}
		response = fileRouteRequest(f.router, http.MethodPost, path, fileRouteToken(t, 8), "", nil)
		assertFileRouteError(t, response, 404, "file not found")
	}
	for _, reader := range []uint{7, 50, 51} {
		if reader == 51 {
			f.users.roles[51] = models.RoleDoctor
		}
		path := fmt.Sprintf("/api/files/%d", file.ID)
		response := fileRouteRequest(f.router, http.MethodGet, path, fileRouteToken(t, reader), "", nil)
		if result := decodeFileRouteResponse(t, response, 200).File; result.UploadedByUserID != 50 {
			t.Fatal("reading must not overwrite the author/uploader")
		}
	}
	response := fileRouteRequest(f.router, http.MethodGet, "/api/files?user_id=50&patient_profile_id=8", f.token, "", nil)
	if files := decodeFileRouteList(t, response); len(files) != 1 || files[0].ID != file.ID {
		t.Fatal("patient must list doctor's report by subject, without honoring spoofed query IDs")
	}
	// Reusing a former doctor's JWT must not grant their old role or uploader-based access.
	f.users.roles[50] = models.RolePatient
	response = fileRouteRequest(f.router, http.MethodGet, fmt.Sprintf("/api/files/%d", file.ID), doctor, "", nil)
	assertFileRouteError(t, response, 404, "file not found")
}

func TestDoctorPatientListingAndAttachmentFiltering(t *testing.T) {
	f := newFileRouteFixture(t)
	doctor := fileRouteToken(t, 50)
	first := uploadDoctorAttachment(t, f, 7, doctor, []byte("patient seven"))
	second := uploadDoctorAttachment(t, f, 8, doctor, []byte("patient eight"))
	// An unassigned historical file must not become visible to every doctor.
	f.repo.files[99] = models.File{ID: 99, UserID: 8, OriginalName: "unassigned-legacy.bin"}
	for _, tt := range []struct {
		path string
		ids  []uint
	}{
		{"/api/doctor/patients/7/files", []uint{first.ID}},
		{"/api/doctor/patients/8/files", []uint{second.ID}},
		{"/api/files", []uint{second.ID, first.ID}},
	} {
		response := fileRouteRequest(f.router, http.MethodGet, tt.path, doctor, "", nil)
		files := decodeFileRouteList(t, response)
		if len(files) != len(tt.ids) {
			t.Fatalf("unexpected result for %s: %+v", tt.path, files)
		}
		for i, id := range tt.ids {
			if files[i].ID != id {
				t.Fatalf("wrong subject/order for %s: %+v", tt.path, files)
			}
		}
	}
	response := fileRouteRequest(f.router, http.MethodGet, "/api/files/99", doctor, "", nil)
	assertFileRouteError(t, response, 404, "file not found")
	response = fileRouteRequest(f.router, http.MethodGet, "/api/files/99", fileRouteToken(t, 8), "", nil)
	legacy := decodeFileRouteResponse(t, response, 200).File
	if legacy.PatientProfileID != nil || legacy.UploadedByUserID != 8 {
		t.Fatal("legacy metadata must show an unassigned patient, not invent ownership")
	}
	response = fileRouteRequest(f.router, http.MethodGet, "/api/doctor/patients", doctor, "", nil)
	if response.Code != 200 || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("doctor directory failed: %d", response.Code)
	}
}

func TestAttachmentUploadTargetAndRoleProtection(t *testing.T) {
	for _, tt := range []struct {
		name, path string
		actor      uint
		status     int
		message    string
	}{
		{"patient on doctor route", "/api/doctor/patients/8/files", 7, 403, "Forbidden"},
		{"doctor without explicit target", "/api/files", 50, 403, "Forbidden"},
		{"zero target", "/api/doctor/patients/0/files", 50, 400, "Invalid patient profile ID"},
		{"malformed target", "/api/doctor/patients/not-an-id/files", 50, 400, "Invalid patient profile ID"},
		{"overflow target", "/api/doctor/patients/18446744073709551615/files", 50, 400, "Invalid patient profile ID"},
		{"doctor is not a patient profile", "/api/doctor/patients/50/files", 50, 404, "Patient profile not found"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFileRouteFixture(t)
			body, contentType := fileRouteMultipart(t, "report.pdf", []byte("private bytes"), 0)
			response := fileRouteRequest(f.router, http.MethodPost, tt.path, fileRouteToken(t, tt.actor), contentType, body)
			assertFileRouteError(t, response, tt.status, tt.message)
			assertFileRouteStorageEmpty(t, f)
		})
	}
	f := newFileRouteFixture(t)
	body, contentType := fileRouteMultipart(t, "own.pdf", []byte("own bytes"), 0)
	response := fileRouteRequest(f.router, http.MethodPost, "/api/files?patient_profile_id=8&user_id=8", f.token, contentType, body)
	own := decodeFileRouteResponse(t, response, 201).File
	if own.PatientProfileID == nil || *own.PatientProfileID != 7 || own.UploadedByUserID != 7 {
		t.Fatal("patient upload must ignore caller-supplied ownership")
	}
}

func TestDoctorFileEndpointsRejectMissingSessionAndInternalFailures(t *testing.T) {
	f := newFileRouteFixture(t)
	for _, path := range []string{"/api/doctor/patients/7/files", "/api/doctor/patients"} {
		response := fileRouteRequest(f.router, http.MethodGet, path, "", "", nil)
		assertFileRouteError(t, response, 401, "Missing authentication cookie")
	}
	f.patients.missing = true
	body, contentType := fileRouteMultipart(t, "report.pdf", []byte("bytes"), 0)
	response := fileRouteRequest(f.router, http.MethodPost, "/api/doctor/patients/7/files", fileRouteToken(t, 50), contentType, body)
	assertFileRouteError(t, response, 404, "Patient profile not found")
	assertFileRouteStorageEmpty(t, f)
	f.patients.missing = false
	f.repo.uploadErr = errors.New("private database details")
	body, contentType = fileRouteMultipart(t, "report.pdf", []byte("bytes"), 0)
	response = fileRouteRequest(f.router, http.MethodPost, "/api/doctor/patients/7/files", fileRouteToken(t, 50), contentType, body)
	assertFileRouteError(t, response, 500, "Failed to process file")
	assertFileRouteStorageEmpty(t, f)
}
