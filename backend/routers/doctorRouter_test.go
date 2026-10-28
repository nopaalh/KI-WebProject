package routers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
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

type doctorDirectoryUsers struct {
	user  *models.User
	err   error
	calls int
	id    uint
}

func (r *doctorDirectoryUsers) FindByID(id uint) (*models.User, error) {
	r.calls++
	r.id = id
	return r.user, r.err
}

// doctorTestKey: kunci master untuk tes direktori dokter.
var doctorTestKey [32]byte

// sealForTest membuat amplop AES1, DES1, dan RC41 dari value, seperti yang dilakukan service.
func sealForTest(key [32]byte, value string) (aes, des, rc4 []byte) {
	var err error
	if aes, err = utils.EncryptAESFile([]byte(value), key); err != nil {
		panic(err)
	}
	if des, err = utils.EncryptDESFile([]byte(value), key); err != nil {
		panic(err)
	}
	if rc4, err = utils.EncryptRC4File([]byte(value), key); err != nil {
		panic(err)
	}
	return aes, des, rc4
}

// patientRowForTest membuat satu baris daftar pasien dengan nama dan username terenkripsi.
func patientRowForTest(key [32]byte, profileID, userID uint, name, username string) repositories.PatientRow {
	row := repositories.PatientRow{PatientProfileID: profileID, UserID: userID}
	row.NameAES, row.NameDES, row.NameRC4 = sealForTest(key, name)
	row.UsernameAES, row.UsernameDES, row.UsernameRC4 = sealForTest(key, username)
	return row
}

type doctorDirectoryPatients struct {
	patients []repositories.PatientRow
	err      error
	calls    int
}

func (r *doctorDirectoryPatients) ListPatients() ([]repositories.PatientRow, error) {
	r.calls++
	return r.patients, r.err
}

func TestDoctorDirectoryAccess(t *testing.T) {
	t.Setenv("JWT_SECRET", "doctor-directory-test-secret")
	token, err := utils.GenerateToken(7, "doctor")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name         string
		token        string
		role         string
		userErr      error
		patientErr   error
		wantStatus   int
		wantUsers    int
		wantPatients int
	}{
		{name: "missing JWT", role: models.RoleDoctor, wantStatus: 401},
		{name: "invalid JWT", token: "invalid", role: models.RoleDoctor, wantStatus: 401},
		{name: "doctor", token: token, role: models.RoleDoctor, wantStatus: 200, wantUsers: 1, wantPatients: 1},
		{name: "patient", token: token, role: models.RolePatient, wantStatus: 403, wantUsers: 1},
		{name: "unknown role", token: token, role: "admin", wantStatus: 403, wantUsers: 1},
		{name: "deleted user", token: token, userErr: gorm.ErrRecordNotFound, wantStatus: 401, wantUsers: 1},
		{name: "user DB error", token: token, userErr: errors.New("private database details"), wantStatus: 500, wantUsers: 1},
		{name: "directory DB error", token: token, role: models.RoleDoctor, patientErr: errors.New("private database details"), wantStatus: 500, wantUsers: 1, wantPatients: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			users := &doctorDirectoryUsers{user: &models.User{ID: 7, Role: tc.role}, err: tc.userErr}
			patients := &doctorDirectoryPatients{err: tc.patientErr}
			controller := controllers.NewDoctorController(services.NewDoctorService(users, patients, doctorTestKey))
			router := gin.New()
			RegisterDoctorRouter(router.Group("/api"), controller, nil)
			req := httptest.NewRequest(http.MethodGet, "/api/doctor/patients", nil)
			if tc.token != "" {
				req.AddCookie(&http.Cookie{Name: "access_token", Value: tc.token})
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != tc.wantStatus {
				t.Fatalf("got %d: %s, want %d", response.Code, response.Body, tc.wantStatus)
			}
			if users.calls != tc.wantUsers || patients.calls != tc.wantPatients {
				t.Fatalf("unexpected authorization/directory calls: %d/%d", users.calls, patients.calls)
			}
			if users.calls > 0 && users.id != 7 {
				t.Fatalf("authorization used wrong user ID: %d", users.id)
			}
			if tc.wantUsers > 0 && response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("directory response must not be cached")
			}
			wantBody := map[int]string{
				200: `{"patients":[]}`, 403: `{"message":"Forbidden"}`, 500: `{"message":"Failed to list patients"}`,
			}
			if want, ok := wantBody[tc.wantStatus]; ok && strings.TrimSpace(response.Body.String()) != want {
				t.Fatalf("unexpected response: %s, want %s", response.Body, want)
			}
		})
	}
}

func TestDoctorDirectoryUsesCurrentRole(t *testing.T) {
	t.Setenv("JWT_SECRET", "doctor-directory-test-secret")
	token, err := utils.GenerateToken(7, "doctor")
	if err != nil {
		t.Fatal(err)
	}
	users := &doctorDirectoryUsers{user: &models.User{ID: 7, Role: models.RoleDoctor}}
	want := []repositories.PatientSummary{{PatientProfileID: 3, UserID: 8, Name: "Patient", Username: "patient"}}
	patients := &doctorDirectoryPatients{patients: []repositories.PatientRow{patientRowForTest(doctorTestKey, 3, 8, "Patient", "patient")}}
	router := gin.New()
	RegisterDoctorRouter(router.Group("/api"), controllers.NewDoctorController(services.NewDoctorService(users, patients, doctorTestKey)), nil)
	for i, status := range []int{http.StatusOK, http.StatusForbidden} {
		req := httptest.NewRequest(http.MethodGet, "/api/doctor/patients", nil)
		req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != status {
			t.Fatalf("request %d: got %d, want %d", i, response.Code, status)
		}
		if i == 0 {
			var body struct {
				Patients []repositories.PatientSummary `json:"patients"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(body.Patients, want) {
				t.Fatalf("unexpected directory: %+v", body.Patients)
			}
		}
		users.user.Role = models.RolePatient
	}
	if users.calls != 2 || patients.calls != 1 {
		t.Fatalf("role must be reloaded once per request before directory read: %d/%d", users.calls, patients.calls)
	}
}

func TestDoctorDirectoryRejectsInvalidContextUserID(t *testing.T) {
	for _, value := range []any{nil, "7", int(7), uint(0)} {
		users := &doctorDirectoryUsers{}
		patients := &doctorDirectoryPatients{}
		controller := controllers.NewDoctorController(services.NewDoctorService(users, patients, doctorTestKey))
		router := gin.New()
		router.GET("/patients", func(ctx *gin.Context) {
			if value != nil {
				ctx.Set("userID", value)
			}
			controller.ListPatients(ctx)
		})
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/patients", nil))
		if response.Code != http.StatusUnauthorized || users.calls != 0 || patients.calls != 0 {
			t.Fatalf("invalid context ID %v must be rejected without reads: %d", value, response.Code)
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("directory response must not be cached")
		}
	}
}
