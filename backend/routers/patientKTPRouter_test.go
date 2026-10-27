package routers

import (
	"encoding/json"
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

type ktpRouteRepository struct {
	stored *models.PatientKTP
	err    error
	calls  int
}

func (r *ktpRouteRepository) Create(ktp *models.PatientKTP) error {
	if r.err != nil {
		return r.err
	}
	if r.stored != nil {
		return repositories.ErrPatientKTPExists
	}
	ktp.ID = 3
	r.stored = ktp
	return nil
}

func (r *ktpRouteRepository) FindByUserID(userID uint) (*models.PatientKTP, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	if userID != 7 || r.stored == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.stored, nil
}

func (r *ktpRouteRepository) FindPhotoByUserID(userID uint) (*models.PatientKTP, error) {
	return r.FindByUserID(userID)
}

func encryptedKTPFixture(t *testing.T, key [32]byte, plaintext []byte) models.KTPEncryptedValue {
	t.Helper()
	var result models.KTPEncryptedValue
	var err error
	result.AES, err = utils.EncryptAESFile(plaintext, key)
	if err != nil {
		t.Fatal(err)
	}
	result.DES, err = utils.EncryptDESFile(plaintext, key)
	if err != nil {
		t.Fatal(err)
	}
	result.RC4, err = utils.EncryptRC4File(plaintext, key)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

type ktpRouteFixture struct {
	router   *gin.Engine
	repo     *ktpRouteRepository
	profiles *profileRouteRepository
	users    *profileRouteUsers
	key      [32]byte
	token    string
}

func newKTPRouteFixture(t *testing.T) *ktpRouteFixture {
	t.Helper()
	t.Setenv("JWT_SECRET", "ktp-route-test-secret")
	f := &ktpRouteFixture{
		repo: &ktpRouteRepository{}, profiles: &profileRouteRepository{},
		users: &profileRouteUsers{role: models.RolePatient}, key: [32]byte{1, 2, 3},
	}
	profiles := services.NewPatientProfileService(f.profiles, f.users, f.key)
	age := 21
	if _, err := profiles.Create(7, services.CreatePatientProfileInput{
		WeightKG: 65.5, HeightCM: 170, Age: &age, NIK: "0000000000000001", BPJSNumber: "0000000000001",
	}); err != nil {
		t.Fatal(err)
	}
	f.users.calls = 0
	text := func(value string) models.KTPEncryptedValue { return encryptedKTPFixture(t, f.key, []byte(value)) }
	// Synthetic read fixture, not an upload. Photo is deliberately absent: this query omits it.
	f.repo.stored = &models.PatientKTP{
		ID: 3, PatientProfileID: 12,
		NIK: text("0000000000000001"), FullName: text("Pasien Sintetis"),
		BirthPlace: text("Kota Contoh"), BirthDate: text("2005-01-01"),
		Gender: text("Laki-laki"), BloodType: text("O"), Address: text("Jalan Contoh 1"),
		RT: text("001"), RW: text("002"), Village: text("Desa Contoh"), District: text("Kecamatan Contoh"),
		Religion: text("Islam"), MaritalStatus: text("Belum Kawin"), Occupation: text("Pelajar"), Citizenship: text("WNI"),
	}
	service := services.NewPatientKTPService(f.repo, profiles, f.key)
	f.router = SetupRouter(Controller{PatientKTPController: controllers.NewPatientKTPController(service)})
	var err error
	f.token, err = utils.GenerateToken(7, "synthetic_patient")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func requestKTP(f *ktpRouteFixture, query, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/patients/me/ktp"+query, nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	}
	response := httptest.NewRecorder()
	f.router.ServeHTTP(response, req)
	return response
}

func TestReadOwnKTPData(t *testing.T) {
	f := newKTPRouteFixture(t)
	response := requestKTP(f, "?user_id=99&patient_profile_id=99", f.token)
	if response.Code != 200 || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("read KTP: %d, %s", response.Code, response.Body.String())
	}
	var data struct {
		KTP map[string]any `json:"ktp"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.KTP) != 17 || data.KTP["nik"] != "0000000000000001" || data.KTP["full_name"] != "Pasien Sintetis" ||
		data.KTP["rt"] != "001" || data.KTP["rw"] != "002" || data.KTP["patient_profile_id"] != float64(12) {
		t.Fatalf("unexpected decrypted KTP DTO: %v", data.KTP)
	}
	if f.users.calls != 1 {
		t.Fatalf("role must be fetched once, got %d lookups", f.users.calls)
	}
}

func TestKTPReadAccessAndMissingData(t *testing.T) {
	for _, scenario := range []string{"missing cookie", "invalid token", "doctor", "deleted user", "other patient", "missing profile", "missing ktp", "database error"} {
		t.Run(scenario, func(t *testing.T) {
			f := newKTPRouteFixture(t)
			token, status := f.token, 500
			switch scenario {
			case "missing cookie":
				token, status = "", 401
			case "invalid token":
				token, status = "invalid", 401
			case "doctor":
				f.users.role, status = models.RoleDoctor, 403
			case "deleted user":
				f.users.err, status = gorm.ErrRecordNotFound, 401
			case "other patient":
				var err error
				token, err = utils.GenerateToken(8, "another_patient")
				if err != nil {
					t.Fatal(err)
				}
				status = 404
			case "missing profile":
				f.profiles.stored, status = nil, 404
			case "missing ktp":
				f.repo.stored, status = nil, 404
			case "database error":
				f.repo.err = errors.New("private database details")
			}
			r := requestKTP(f, "?user_id=7&patient_profile_id=12", token)
			if r.Code != status || strings.Contains(r.Body.String(), "Pasien Sintetis") || strings.Contains(r.Body.String(), "private database details") {
				t.Fatalf("unauthorized/error response leaked data: %d, %s", r.Code, r.Body.String())
			}
			if (scenario == "doctor" || scenario == "deleted user" || scenario == "other patient") && f.repo.calls != 0 {
				t.Fatal("must reject before querying KTP storage")
			}
		})
	}
}

func TestKTPReadRejectsInvalidCiphertext(t *testing.T) {
	for _, scenario := range []string{"aes tamper", "des tamper", "rc4 tamper", "missing rc4", "mismatched plaintext", "nik mismatch", "wrong profile", "invalid content", "wrong key"} {
		t.Run(scenario, func(t *testing.T) {
			f := newKTPRouteFixture(t)
			target := &f.repo.stored.FullName
			switch scenario {
			case "aes tamper":
				target.AES[len(target.AES)-1] ^= 1
			case "des tamper":
				target.DES[len(target.DES)-1] ^= 1
			case "rc4 tamper":
				target.RC4[len(target.RC4)-1] ^= 1
			case "missing rc4":
				target.RC4 = nil
			case "mismatched plaintext":
				var err error
				target.RC4, err = utils.EncryptRC4File([]byte("different plaintext"), f.key)
				if err != nil {
					t.Fatal(err)
				}
			case "nik mismatch":
				f.repo.stored.NIK = encryptedKTPFixture(t, f.key, []byte("0000000000000002"))
			case "wrong profile":
				f.repo.stored.PatientProfileID = 99
			case "invalid content":
				*target = encryptedKTPFixture(t, f.key, []byte{0xff})
			case "wrong key":
				wrongKey := f.key
				wrongKey[0] ^= 1
				*target = encryptedKTPFixture(t, wrongKey, []byte("private value"))
			}
			r := requestKTP(f, "", f.token)
			if r.Code != 500 || strings.Contains(r.Body.String(), `"ktp":`) || strings.Contains(r.Body.String(), "Pasien Sintetis") {
				t.Fatalf("corruption must not return partial KTP: %d, %s", r.Code, r.Body.String())
			}
		})
	}
}
