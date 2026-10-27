package routers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"project1/models"
	"project1/utils"

	"gorm.io/gorm"
)

func readOwnProfile(router http.Handler, token, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/patients/me/profile"+query, nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}

func TestReadOwnPatientProfile(t *testing.T) {
	router, _, users, _, token := profileRouteFixture(t)
	if r := sendProfile(router, token, validProfileJSON, "application/json"); r.Code != http.StatusCreated {
		t.Fatalf("create fixture: %d", r.Code)
	}
	users.calls = 0
	// Caller-supplied IDs cannot select a different patient's profile.
	response := readOwnProfile(router, token, "?user_id=99&patient_profile_id=99")
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("read: %d, %s", response.Code, response.Body.String())
	}
	var data struct {
		Profile map[string]any `json:"profile"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	p := data.Profile
	if len(p) != 7 || p["id"] != float64(12) || p["user_id"] != float64(7) ||
		p["weight_kg"] != 65.5 || p["height_cm"] != float64(170) || p["age"] != float64(0) ||
		p["nik"] != "0000000000000001" || p["bpjs_number"] != "0000000000001" {
		t.Fatalf("unexpected profile DTO: %v", p)
	}
	if users.calls != 1 {
		t.Fatalf("GET must load the current role exactly once, got %d queries", users.calls)
	}
	otherToken, err := utils.GenerateToken(8, "another_patient")
	if err != nil {
		t.Fatal(err)
	}
	other := readOwnProfile(router, otherToken, "?user_id=7&patient_profile_id=12")
	if other.Code != http.StatusNotFound || strings.Contains(other.Body.String(), "0000000000000001") {
		t.Fatalf("another patient must not access this profile: %d", other.Code)
	}
}

func TestReadPatientProfileAccessAndErrors(t *testing.T) {
	for _, tt := range []struct {
		name    string
		token   string
		role    string
		userErr error
		findErr error
		status  int
	}{
		{"missing cookie", "", models.RolePatient, nil, nil, 401},
		{"invalid token", "invalid", models.RolePatient, nil, nil, 401},
		{"doctor", "valid", models.RoleDoctor, nil, nil, 403},
		{"unknown role", "valid", "admin", nil, nil, 403},
		{"deleted user", "valid", models.RolePatient, gorm.ErrRecordNotFound, nil, 401},
		{"no profile", "valid", models.RolePatient, nil, nil, 404},
		{"user lookup error", "valid", models.RolePatient, errors.New("private database details"), nil, 500},
		{"profile lookup error", "valid", models.RolePatient, nil, errors.New("private database details"), 500},
	} {
		t.Run(tt.name, func(t *testing.T) {
			router, repo, users, _, token := profileRouteFixture(t)
			users.role, users.err, repo.findErr = tt.role, tt.userErr, tt.findErr
			if tt.token != "valid" {
				token = tt.token
			}
			response := readOwnProfile(router, token, "")
			if response.Code != tt.status {
				t.Fatalf("want %d, got %d: %s", tt.status, response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "private database details") || strings.Contains(response.Body.String(), `"profile":`) {
				t.Fatal("failure must not leak private details or partial profile data")
			}
		})
	}
}

func TestReadPatientProfileRejectsCorruption(t *testing.T) {
	for _, field := range []string{"nik_aes", "nik_des", "nik_rc4", "bpjs_aes", "bpjs_des", "bpjs_rc4", "missing_des", "partial_rc4", "empty_rc4", "different_rc4_plaintext", "wrong_key"} {
		t.Run(field, func(t *testing.T) {
			router, repo, _, key, token := profileRouteFixture(t)
			if r := sendProfile(router, token, validProfileJSON, "application/json"); r.Code != 201 {
				t.Fatalf("create fixture: %d", r.Code)
			}
			p := repo.stored
			fields := map[string][]byte{
				"nik_aes": p.NIKAES, "nik_des": p.NIKDES, "nik_rc4": p.NIKRC4,
				"bpjs_aes": p.BPJSNumberAES, "bpjs_des": p.BPJSNumberDES, "bpjs_rc4": p.BPJSNumberRC4,
			}
			switch field {
			case "missing_des":
				p.NIKDES, p.NIKRC4, p.BPJSNumberRC4 = nil, nil, nil
			case "partial_rc4":
				p.NIKRC4 = nil
			case "empty_rc4":
				p.NIKRC4 = []byte{}
			case "different_rc4_plaintext", "wrong_key":
				value := "0000000000000002"
				if field == "wrong_key" {
					key[0] ^= 1
					value = "0000000000000001"
				}
				var err error
				p.NIKRC4, err = utils.EncryptRC4File([]byte(value), key)
				if err != nil {
					t.Fatal(err)
				}
			default:
				data := fields[field]
				data[len(data)-1] ^= 1
			}
			response := readOwnProfile(router, token, "")
			if response.Code != 500 || strings.Contains(response.Body.String(), `"profile":`) || strings.Contains(response.Body.String(), "0000000000000001") {
				t.Fatalf("corruption must fail without partial data: %d, %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestReadLegacyPatientProfileAndCurrentRole(t *testing.T) {
	router, repo, users, _, token := profileRouteFixture(t)
	if r := sendProfile(router, token, validProfileJSON, "application/json"); r.Code != 201 {
		t.Fatalf("create fixture: %d", r.Code)
	}
	repo.stored.NIKRC4, repo.stored.BPJSNumberRC4 = nil, nil
	if r := readOwnProfile(router, token, ""); r.Code != 200 {
		t.Fatalf("legacy AES/DES profile should remain readable: %d", r.Code)
	}
	if repo.stored.NIKRC4 != nil || repo.stored.BPJSNumberRC4 != nil {
		t.Fatal("GET must not backfill or mutate a legacy profile")
	}
	users.role = models.RoleDoctor
	if r := readOwnProfile(router, token, ""); r.Code != 403 {
		t.Fatalf("same JWT must use the current DB role: %d", r.Code)
	}
}
