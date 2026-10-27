package routers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
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

func TestLogoutClearsCookieWithoutRequiringValidSession(t *testing.T) {
	// Logout must work even without a JWT secret or a functioning database.
	t.Setenv("JWT_SECRET", "")
	router := newUserTestRouter(&memoryUserRepository{findErr: errors.New("database unavailable")})
	for _, token := range []string{"", "invalid-or-expired-token"} {
		req := httptest.NewRequest(http.MethodPost, "/api/users/logout", nil)
		if token != "" {
			req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != http.StatusOK || response.Body.String() != `{"message":"Logout successful"}` {
			t.Fatalf("logout must be idempotent: %d, %s", response.Code, response.Body.String())
		}
		cookies := response.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("expected one deletion cookie, got %d", len(cookies))
		}
		cookie := cookies[0]
		if cookie.Name != "access_token" || cookie.Value != "" || cookie.MaxAge >= 0 || cookie.Path != "/" ||
			cookie.Domain != "" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Secure {
			t.Fatalf("deletion cookie must match current login attributes: %+v", cookie)
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("logout response must not be cached")
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/api/users/logout", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code == http.StatusOK || len(response.Result().Cookies()) != 0 {
		t.Fatal("GET must not perform logout")
	}
}

func TestLoginLogoutCookieJarFlow(t *testing.T) {
	t.Setenv("JWT_SECRET", "logout-flow-test-secret")
	hash, err := bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	router := newUserTestRouter(&memoryUserRepository{user: models.User{
		ID: 1, Name: "Synthetic User", Username: "demo", PasswordHash: string(hash), Role: models.RolePatient,
	}})
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	base, err := url.Parse("http://example.test/api/users/")
	if err != nil {
		t.Fatal(err)
	}
	login := httptest.NewRequest(http.MethodPost, base.String()+"login", strings.NewReader(`{"username":"demo","password":"test-password"}`))
	login.Header.Set("Content-Type", "application/json")
	loginResponse := httptest.NewRecorder()
	router.ServeHTTP(loginResponse, login)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login: %d", loginResponse.Code)
	}
	loginCookies := loginResponse.Result().Cookies()
	jar.SetCookies(base, loginCookies)
	if len(jar.Cookies(base)) != 1 {
		t.Fatal("login cookie was not stored")
	}
	logout := httptest.NewRequest(http.MethodPost, base.String()+"logout", nil)
	for _, cookie := range jar.Cookies(base) {
		logout.AddCookie(cookie)
	}
	logoutResponse := httptest.NewRecorder()
	router.ServeHTTP(logoutResponse, logout)
	jar.SetCookies(base, logoutResponse.Result().Cookies())
	if logoutResponse.Code != http.StatusOK || len(jar.Cookies(base)) != 0 {
		t.Fatal("logout must remove the login cookie from the browser jar")
	}
	me := httptest.NewRequest(http.MethodGet, base.String()+"me", nil)
	for _, cookie := range jar.Cookies(base) {
		me.AddCookie(cookie)
	}
	meResponse := httptest.NewRecorder()
	router.ServeHTTP(meResponse, me)
	if meResponse.Code != http.StatusUnauthorized {
		t.Fatalf("protected endpoint must reject after cookie deletion: %d", meResponse.Code)
	}
	// Cookie deletion is not server-side JWT revocation; an independent copy still validates.
	if len(loginCookies) != 1 {
		t.Fatal("expected one login cookie")
	}
	if _, err := utils.ValidateToken(loginCookies[0].Value); err != nil {
		t.Fatalf("logout unexpectedly changed JWT validation: %v", err)
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
