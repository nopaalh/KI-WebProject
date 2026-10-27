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
	"project1/services"
	"project1/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type memoryUserRepository struct {
	user    models.User
	findErr error
}

func (r *memoryUserRepository) Create(user *models.User) error {
	user.ID = 1
	r.user = *user
	return nil
}

func (r *memoryUserRepository) FindByUsername(username string) (*models.User, error) {
	if r.user.Username != username {
		return nil, gorm.ErrRecordNotFound
	}
	return &r.user, nil
}

func (r *memoryUserRepository) FindByID(id uint) (*models.User, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.user.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return &r.user, nil
}

func newUserTestRouter(repo *memoryUserRepository) *gin.Engine {
	router := gin.New()
	RegisterUserRouter(router.Group("/api"), controllers.NewAuthController(services.NewAuthService(repo)))
	return router
}

func TestPublicRegistrationAlwaysCreatesPatient(t *testing.T) {
	for _, extra := range []string{"", `,"role":"doctor"`, `,"role":"admin"`} {
		t.Run(extra, func(t *testing.T) {
			repo := &memoryUserRepository{}
			router := newUserTestRouter(repo)
			body := `{"name":"Synthetic Patient","username":"patient_demo","password":"test-password"` + extra + `}`
			req := httptest.NewRequest(http.MethodPost, "/api/users/register", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != http.StatusCreated {
				t.Fatalf("register: status %d, body %s", response.Code, response.Body.String())
			}
			if repo.user.Role != models.RolePatient {
				t.Fatalf("public registration created role %q", repo.user.Role)
			}
			if err := bcrypt.CompareHashAndPassword([]byte(repo.user.PasswordHash), []byte("test-password")); err != nil {
				t.Fatalf("password was not hashed correctly: %v", err)
			}
		})
	}
}

func TestMeReadsCurrentDatabaseRole(t *testing.T) {
	t.Setenv("JWT_SECRET", "user-router-test-secret")
	repo := &memoryUserRepository{user: models.User{
		ID: 1, Name: "Synthetic User", Username: "demo", PasswordHash: "never-expose-this", Role: models.RolePatient,
	}}
	router := newUserTestRouter(repo)

	// Use the real login endpoint and its HttpOnly cookie for both roles.
	hash, err := bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	repo.user.PasswordHash = string(hash)
	login := httptest.NewRequest(http.MethodPost, "/api/users/login", strings.NewReader(`{"username":"demo","password":"test-password"}`))
	login.Header.Set("Content-Type", "application/json")
	loginResponse := httptest.NewRecorder()
	router.ServeHTTP(loginResponse, login)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login: status %d", loginResponse.Code)
	}
	cookies := loginResponse.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "access_token" || !cookies[0].HttpOnly {
		t.Fatal("login did not return the expected HttpOnly authentication cookie")
	}

	for _, role := range []string{models.RolePatient, models.RoleDoctor, models.RolePatient} {
		repo.user.Role = role
		req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
		req.AddCookie(cookies[0])
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != http.StatusOK {
			t.Fatalf("me: status %d, body %s", response.Code, response.Body.String())
		}
		var data map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		if len(data) != 4 || data["role"] != role || data["id"] != float64(1) || data["name"] != "Synthetic User" || data["username"] != "demo" {
			t.Fatalf("unexpected public profile: %v", data)
		}
	}
}

func TestMeAuthenticationAndLookupErrors(t *testing.T) {
	t.Setenv("JWT_SECRET", "user-router-test-secret")
	token, err := utils.GenerateToken(1, "demo")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name    string
		token   string
		findErr error
		status  int
	}{
		{"missing cookie", "", nil, http.StatusUnauthorized},
		{"invalid token", "invalid", nil, http.StatusUnauthorized},
		{"deleted user", token, gorm.ErrRecordNotFound, http.StatusUnauthorized},
		{"database failure", token, errors.New("private database details"), http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			router := newUserTestRouter(&memoryUserRepository{findErr: tt.findErr})
			req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
			if tt.token != "" {
				req.AddCookie(&http.Cookie{Name: "access_token", Value: tt.token})
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != tt.status {
				t.Fatalf("status %d, want %d", response.Code, tt.status)
			}
			if strings.Contains(response.Body.String(), "private database details") {
				t.Fatal("response leaked database details")
			}
		})
	}
}
