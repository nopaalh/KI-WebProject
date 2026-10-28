package routers

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"project1/controllers"
	"project1/models"
	"project1/repositories"
	"project1/services"
	"project1/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const validProfileJSON = `{"weight_kg":65.5,"height_cm":170,"age":0,"nik":"0000000000000001","bpjs_number":"0000000000001"}`

type profileRouteRepository struct {
	stored  *models.PatientProfile
	err     error
	findErr error
}

func (r *profileRouteRepository) Create(profile *models.PatientProfile) error {
	if r.err != nil {
		return r.err
	}
	if r.stored != nil {
		return repositories.ErrPatientProfileExists
	}
	profile.ID = 12
	r.stored = profile
	return nil
}

func (r *profileRouteRepository) FindByUserID(userID uint) (*models.PatientProfile, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.stored == nil || r.stored.UserID != userID {
		return nil, gorm.ErrRecordNotFound
	}
	return r.stored, nil
}

type profileRouteUsers struct {
	role  string
	err   error
	calls int
}

func (r *profileRouteUsers) FindByID(id uint) (*models.User, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	return &models.User{ID: id, Role: r.role}, nil
}

func profileRouteFixture(t *testing.T) (*gin.Engine, *profileRouteRepository, *profileRouteUsers, [32]byte, string) {
	t.Helper()
	t.Setenv("JWT_SECRET", "patient-profile-route-test-secret")
	repo := &profileRouteRepository{}
	users := &profileRouteUsers{role: models.RolePatient}
	key := [32]byte{1, 2, 3}
	service := services.NewPatientProfileService(repo, users, key)
	router := SetupRouter(Controller{PatientProfileController: controllers.NewPatientProfileController(service)})
	token, err := utils.GenerateToken(7, "synthetic_patient")
	if err != nil {
		t.Fatal(err)
	}
	return router, repo, users, key, token
}

func sendProfile(router http.Handler, token, body, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/patients/me/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}

func TestCreatePatientProfileEncryptsAESDESAndRC4(t *testing.T) {
	router, repo, users, key, token := profileRouteFixture(t)
	response := sendProfile(router, token, validProfileJSON, "application/json")
	if response.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	p := repo.stored
	if p == nil || p.UserID != 7 || p.UserRole != models.RolePatient {
		t.Fatal("expected authenticated patient's profile")
	}
	if users.calls != 1 {
		t.Fatalf("role must be loaded from the database once, got %d lookups", users.calls)
	}

	for _, tt := range []struct {
		value string
		aes   []byte
		des   []byte
		rc4   []byte
	}{{"0000000000000001", p.NIKAES, p.NIKDES, p.NIKRC4}, {"0000000000001", p.BPJSNumberAES, p.BPJSNumberDES, p.BPJSNumberRC4}} {
		aes, err := utils.DecryptAESFile(tt.aes, key)
		if err != nil || string(aes) != tt.value {
			t.Fatal("AES did not round-trip identically")
		}
		des, err := utils.DecryptDESFile(tt.des, key)
		if err != nil || string(des) != tt.value {
			t.Fatal("DES did not round-trip identically")
		}
		rc4, err := utils.DecryptRC4File(tt.rc4, key)
		if err != nil || string(rc4) != tt.value || !bytes.HasPrefix(tt.rc4, []byte("RC41")) {
			t.Fatal("RC4 did not store a valid envelope and round-trip identically")
		}
		tampered := bytes.Clone(tt.rc4)
		tampered[len(tampered)-1] ^= 1
		if decoded, err := utils.DecryptRC4File(tampered, key); err == nil || decoded != nil {
			t.Fatal("tampered stored RC4 ciphertext must be rejected without plaintext")
		}
		if bytes.Contains(response.Body.Bytes(), []byte(tt.value)) {
			t.Fatal("response leaked an identity number")
		}
	}

	for _, tt := range []struct {
		value         string
		aes, des, rc4 []byte
	}{{"65.50", p.WeightKGAES, p.WeightKGDES, p.WeightKGRC4}, {"170.00", p.HeightCMAES, p.HeightCMDES, p.HeightCMRC4}, {"0", p.AgeAES, p.AgeDES, p.AgeRC4}} {
		aes, errAES := utils.DecryptAESFile(tt.aes, key)
		des, errDES := utils.DecryptDESFile(tt.des, key)
		rc4, errRC4 := utils.DecryptRC4File(tt.rc4, key)
		if errAES != nil || errDES != nil || errRC4 != nil || string(aes) != tt.value || string(des) != tt.value || string(rc4) != tt.value {
			t.Fatalf("measurement %s did not round-trip through all three ciphers", tt.value)
		}
	}
	if !strings.Contains(response.Body.String(), `"active":["aes","des","rc4"]`) || !strings.Contains(response.Body.String(), `"pending":[]`) {
		t.Fatal("response must disclose all three active ciphers without pending RC4")
	}
	if second := sendProfile(router, token, validProfileJSON, "application/json"); second.Code != http.StatusConflict {
		t.Fatalf("duplicate profile must return 409, got %d", second.Code)
	}
}

func TestCreatePatientProfileRejectsInvalidRequests(t *testing.T) {
	for _, tt := range []struct {
		name, body string
	}{
		{"missing age", strings.Replace(validProfileJSON, `"age":0,`, "", 1)},
		{"null age", strings.Replace(validProfileJSON, `"age":0`, `"age":null`, 1)},
		{"negative age", strings.Replace(validProfileJSON, `"age":0`, `"age":-1`, 1)},
		{"age too large", strings.Replace(validProfileJSON, `"age":0`, `"age":151`, 1)},
		{"zero weight", strings.Replace(validProfileJSON, "65.5", "0", 1)},
		{"negative height", strings.Replace(validProfileJSON, "170", "-1", 1)},
		{"numeric overflow", strings.Replace(validProfileJSON, "65.5", "10000", 1)},
		{"extra precision", strings.Replace(validProfileJSON, "65.5", "65.555", 1)},
		{"invalid nik", strings.Replace(validProfileJSON, "0000000000000001", "123", 1)},
		{"non-digit nik", strings.Replace(validProfileJSON, "0000000000000001", "000000000000000a", 1)},
		{"invalid bpjs", strings.Replace(validProfileJSON, "0000000000001", "123", 1)},
		{"numeric nik", strings.Replace(validProfileJSON, `"nik":"0000000000000001"`, `"nik":123`, 1)},
		{"spoofed owner", strings.Replace(validProfileJSON, "{", `{"user_id":99,`, 1)},
		{"spoofed role", strings.Replace(validProfileJSON, "{", `{"role":"doctor",`, 1)},
		{"multiple objects", validProfileJSON + `{}`},
		{"malformed json", `{`},
		{"null object", `null`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			router, repo, _, _, token := profileRouteFixture(t)
			response := sendProfile(router, token, tt.body, "application/json")
			if response.Code != http.StatusBadRequest || repo.stored != nil {
				t.Fatalf("want 400 without persistence, got %d: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestCreatePatientProfileAccessAndFailures(t *testing.T) {
	for _, tt := range []struct {
		name        string
		role        string
		userErr     error
		saveErr     error
		withoutAuth bool
		status      int
	}{
		{"missing cookie", models.RolePatient, nil, nil, true, 401},
		{"doctor", models.RoleDoctor, nil, nil, false, 403},
		{"unknown role", "admin", nil, nil, false, 403},
		{"deleted user", models.RolePatient, gorm.ErrRecordNotFound, nil, false, 401},
		{"user lookup failure", models.RolePatient, errors.New("private DB error"), nil, false, 500},
		{"insert failure", models.RolePatient, nil, errors.New("private DB error"), false, 500},
	} {
		t.Run(tt.name, func(t *testing.T) {
			router, repo, users, _, token := profileRouteFixture(t)
			users.role, users.err, repo.err = tt.role, tt.userErr, tt.saveErr
			if tt.withoutAuth {
				token = ""
			}
			response := sendProfile(router, token, validProfileJSON, "application/json")
			if response.Code != tt.status || repo.stored != nil {
				t.Fatalf("want %d without persistence, got %d", tt.status, response.Code)
			}
			if strings.Contains(response.Body.String(), "private DB error") {
				t.Fatal("response leaked database details")
			}
		})
	}
}

func TestCreatePatientProfileBodyLimitsAndCurrentRole(t *testing.T) {
	router, repo, users, _, token := profileRouteFixture(t)
	if r := sendProfile(router, token, validProfileJSON, "text/plain"); r.Code != 415 {
		t.Fatalf("expected 415, got %d", r.Code)
	}
	body := strings.Replace(validProfileJSON, "0000000000000001", strings.Repeat("0", 5000), 1)
	if r := sendProfile(router, token, body, "application/json"); r.Code != 413 {
		t.Fatalf("expected 413, got %d", r.Code)
	}
	if r := sendProfile(router, "invalid-token", validProfileJSON, "application/json"); r.Code != 401 {
		t.Fatalf("expected 401, got %d", r.Code)
	}
	// The same JWT must not preserve a patient's former role after a DB role change.
	users.role = models.RoleDoctor
	if r := sendProfile(router, token, validProfileJSON, "application/json"); r.Code != 403 || repo.stored != nil {
		t.Fatalf("expected current database role to deny creation, got %d", r.Code)
	}
}
