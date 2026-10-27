package routers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"project1/controllers"
	"project1/models"
	"project1/services"
	"project1/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type memoryFileRepository struct {
	files     map[uint]models.File
	uploadErr error
	listErr   error
	findErr   error
	updateErr error
}

func (r *memoryFileRepository) UploadFile(file *models.File) error {
	if r.uploadErr != nil {
		return r.uploadErr
	}
	file.ID = uint(len(r.files) + 1)
	r.files[file.ID] = *file
	return nil
}

func (r *memoryFileRepository) ListByUserID(userID uint) ([]models.File, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	files := make([]models.File, 0)
	for _, file := range r.files {
		if file.UserID == userID {
			files = append(files, file)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].ID > files[j].ID })
	return files, nil
}

func (r *memoryFileRepository) FindByIDAndUserID(fileID, userID uint) (*models.File, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	file, ok := r.files[fileID]
	if !ok || file.UserID != userID {
		return nil, gorm.ErrRecordNotFound
	}
	return &file, nil
}

func (r *memoryFileRepository) UpdateDecryptTime(fileID, userID uint, algorithm string, durationNS int64) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	file, ok := r.files[fileID]
	if !ok || file.UserID != userID {
		return gorm.ErrRecordNotFound
	}
	switch algorithm {
	case "aes":
		file.AESDecryptTimeNS = durationNS
	case "des":
		file.DESDecryptTimeNS = durationNS
	case "rc4":
		file.RC4DecryptTimeNS = durationNS
	default:
		return services.ErrInvalidFileAlgorithm
	}
	r.files[fileID] = file
	return nil
}

type fileRouteFixture struct {
	router     *gin.Engine
	controller *controllers.FileController
	repo       *memoryFileRepository
	storageDir string
	token      string
}

func newFileRouteFixture(t *testing.T) *fileRouteFixture {
	t.Helper()
	t.Setenv("JWT_SECRET", "file-router-integration-test-secret")
	repo := &memoryFileRepository{files: make(map[uint]models.File)}
	storageDir := t.TempDir()
	var key [32]byte
	for i := range key {
		key[i] = byte(i + 1)
	}
	service, err := services.NewFileService(repo, storageDir, key)
	if err != nil {
		t.Fatalf("create real file service: %v", err)
	}
	controller := controllers.NewFileController(service)
	router := gin.New()
	RegisterFileRouter(router.Group("/api"), controller)
	return &fileRouteFixture{
		router: router, controller: controller, repo: repo,
		storageDir: storageDir, token: fileRouteToken(t, 7),
	}
}

func fileRouteToken(t *testing.T, userID uint) string {
	t.Helper()
	token, err := utils.GenerateToken(userID, fmt.Sprintf("user-%d", userID))
	if err != nil {
		t.Fatalf("generate authentication token: %v", err)
	}
	return token
}

func fileRouteRequest(router http.Handler, method, path, token, contentType string, body io.Reader) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}

func fileRouteMultipart(t *testing.T, filename string, data []byte, padding int) (*bytes.Buffer, string) {
	t.Helper()
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	if filename != "" {
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("create file part: %v", err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatalf("write file part: %v", err)
		}
	}
	if padding > 0 {
		if err := writer.WriteField("padding", strings.Repeat("x", padding)); err != nil {
			t.Fatalf("write multipart field: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return body, writer.FormDataContentType()
}

type fileRouteMetadata struct {
	ID               uint     `json:"id"`
	OriginalName     string   `json:"original_name"`
	OriginalSize     int64    `json:"original_size"`
	OriginalType     string   `json:"original_type"`
	Algorithm        string   `json:"algorithm"`
	Algorithms       []string `json:"algorithms"`
	CipherSetVersion int      `json:"cipher_set_version"`
	DESEncryptTimeNS int64    `json:"des_encrypt_time_ns"`
	DESDecryptTimeNS int64    `json:"des_decrypt_time_ns"`
	RC4EncryptTimeNS int64    `json:"rc4_encrypt_time_ns"`
	RC4DecryptTimeNS int64    `json:"rc4_decrypt_time_ns"`
	AESEncryptTimeNS int64    `json:"aes_encrypt_time_ns"`
	AESDecryptTimeNS int64    `json:"aes_decrypt_time_ns"`
}

type fileRouteResponse struct {
	Message string            `json:"message"`
	File    fileRouteMetadata `json:"file"`
}

func decodeFileRouteResponse(t *testing.T, response *httptest.ResponseRecorder, status int) fileRouteResponse {
	t.Helper()
	if response.Code != status {
		t.Fatalf("HTTP status = %d, want %d; body: %s", response.Code, status, response.Body.String())
	}
	mediaType, _, err := mime.ParseMediaType(response.Header().Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json (error: %v)", response.Header().Get("Content-Type"), err)
	}
	var result fileRouteResponse
	decoder := json.NewDecoder(response.Body)
	// An allowlisted response schema also rejects leaked storage paths or keys.
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		t.Fatalf("decode public JSON response: %v", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		t.Fatalf("unexpected trailing response data: %v", err)
	}
	return result
}

func assertFileRouteError(t *testing.T, response *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("HTTP status = %d, want %d; body: %s", response.Code, status, response.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if len(result) != 1 || result["message"] != message {
		t.Fatalf("error response = %s, want only message %q", response.Body.String(), message)
	}
}

func uploadFileRoute(t *testing.T, fixture *fileRouteFixture, filename string, data []byte) fileRouteMetadata {
	t.Helper()
	body, contentType := fileRouteMultipart(t, filename, data, 0)
	response := fileRouteRequest(fixture.router, http.MethodPost, "/api/files", fixture.token, contentType, body)
	result := decodeFileRouteResponse(t, response, http.StatusCreated)
	if result.Message != "File encrypted and saved successfully" {
		t.Fatalf("upload message = %q", result.Message)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("upload must disable caching")
	}
	return result.File
}

func assertFileRouteStorageEmpty(t *testing.T, fixture *fileRouteFixture) {
	t.Helper()
	if len(fixture.repo.files) != 0 {
		t.Fatalf("rejected request persisted %d records", len(fixture.repo.files))
	}
	entries, err := os.ReadDir(fixture.storageDir)
	if err != nil {
		t.Fatalf("read storage directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("rejected request left %d files on disk", len(entries))
	}
}

func TestFileRoutesRequireAuthentication(t *testing.T) {
	fixture := newFileRouteFixture(t)
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/api/files"},
		{http.MethodGet, "/api/files"},
		{http.MethodGet, "/api/files/1"},
		{http.MethodPost, "/api/files/1/decrypt"},
	} {
		for _, auth := range []struct{ name, token, message string }{
			{"missing cookie", "", "Missing authentication cookie"},
			{"invalid cookie", "not-a-valid-jwt", "Invalid or expired token"},
		} {
			t.Run(route.method+" "+route.path+"/"+auth.name, func(t *testing.T) {
				response := fileRouteRequest(fixture.router, route.method, route.path, auth.token, "", nil)
				assertFileRouteError(t, response, http.StatusUnauthorized, auth.message)
			})
		}
	}
	assertFileRouteStorageEmpty(t, fixture)
}

func TestFileRoutesUploadMetadataAndDecrypt(t *testing.T) {
	fixture := newFileRouteFixture(t)
	filename := `report "Q4".bin`
	plaintext := bytes.Repeat([]byte{0, 1, 2, '\n', '\r', 127, 128, 255}, 512)
	metadata := uploadFileRoute(t, fixture, filename, plaintext)
	if metadata.ID == 0 || metadata.OriginalName != filename || metadata.OriginalSize != int64(len(plaintext)) ||
		metadata.OriginalType != "application/octet-stream" || metadata.Algorithm != services.AESFileAlgorithm {
		t.Fatalf("unexpected upload metadata: %+v", metadata)
	}
	if metadata.AESEncryptTimeNS <= 0 || metadata.AESDecryptTimeNS <= 0 {
		t.Fatalf("expected positive nanosecond timings: %+v", metadata)
	}
	stored := fixture.repo.files[metadata.ID]
	if stored.UserID != 7 || filepath.Dir(stored.AESPath) != fixture.storageDir {
		t.Fatalf("unexpected persisted owner/storage path: %+v", stored)
	}
	ciphertext, err := os.ReadFile(stored.AESPath)
	if err != nil {
		t.Fatalf("read encrypted file: %v", err)
	}
	if len(ciphertext) == 0 || bytes.Equal(ciphertext, plaintext) {
		t.Fatal("disk file must contain encrypted data, not plaintext")
	}

	path := fmt.Sprintf("/api/files/%d", metadata.ID)
	response := fileRouteRequest(fixture.router, http.MethodGet, path, fixture.token, "", nil)
	if got := decodeFileRouteResponse(t, response, http.StatusOK); !reflect.DeepEqual(got.File, metadata) || got.Message != "" {
		t.Fatalf("GET metadata = %+v, want %+v", got, metadata)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("metadata must disable caching")
	}

	response = fileRouteRequest(fixture.router, http.MethodPost, path+"/decrypt", fixture.token, "", nil)
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), plaintext) {
		t.Fatalf("decrypt status = %d, exact plaintext match = %t", response.Code, bytes.Equal(response.Body.Bytes(), plaintext))
	}
	if response.Header().Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("decrypt Content-Type = %q", response.Header().Get("Content-Type"))
	}
	disposition, params, err := mime.ParseMediaType(response.Header().Get("Content-Disposition"))
	if err != nil || disposition != "attachment" || params["filename"] != filename {
		t.Fatalf("invalid attachment header %q (error: %v)", response.Header().Get("Content-Disposition"), err)
	}
	duration, err := strconv.ParseInt(response.Header().Get("X-Decryption-Time-Ns"), 10, 64)
	if err != nil || duration <= 0 || duration != fixture.repo.files[metadata.ID].AESDecryptTimeNS {
		t.Fatalf("decryption duration = %d (error: %v), persisted = %d", duration, err, fixture.repo.files[metadata.ID].AESDecryptTimeNS)
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("decrypted download must disable caching and MIME sniffing")
	}
	response = fileRouteRequest(fixture.router, http.MethodGet, path, fixture.token, "", nil)
	if got := decodeFileRouteResponse(t, response, http.StatusOK).File; got.AESDecryptTimeNS != duration {
		t.Fatalf("GET decryption timing = %d, want %d", got.AESDecryptTimeNS, duration)
	}
}

func TestFileRoutesNotFoundAndOwnership(t *testing.T) {
	fixture := newFileRouteFixture(t)
	metadata := uploadFileRoute(t, fixture, "private.bin", []byte("owner-only content"))
	original := fixture.repo.files[metadata.ID]
	for _, test := range []struct {
		name  string
		id    uint
		token string
	}{
		{"other user", metadata.ID, fileRouteToken(t, 8)},
		{"missing record", metadata.ID + 1, fixture.token},
	} {
		for _, route := range []struct{ method, suffix string }{{http.MethodGet, ""}, {http.MethodPost, "/decrypt"}} {
			t.Run(test.name+route.suffix, func(t *testing.T) {
				path := fmt.Sprintf("/api/files/%d%s", test.id, route.suffix)
				response := fileRouteRequest(fixture.router, route.method, path, test.token, "", nil)
				assertFileRouteError(t, response, http.StatusNotFound, "file not found")
			})
		}
	}
	if fixture.repo.files[metadata.ID] != original {
		t.Fatal("not-found requests must not modify the owner's metadata")
	}
}

func TestFileRoutesInvalidID(t *testing.T) {
	fixture := newFileRouteFixture(t)
	for _, id := range []string{"not-an-id", "0", "-1", "18446744073709551616"} {
		for _, route := range []struct{ method, suffix string }{{http.MethodGet, ""}, {http.MethodPost, "/decrypt"}} {
			t.Run(id+route.suffix, func(t *testing.T) {
				response := fileRouteRequest(fixture.router, route.method, "/api/files/"+id+route.suffix, fixture.token, "", nil)
				assertFileRouteError(t, response, http.StatusBadRequest, "Invalid file ID")
			})
		}
	}
	assertFileRouteStorageEmpty(t, fixture)
}

func TestFileRoutesInvalidMultipart(t *testing.T) {
	fixture := newFileRouteFixture(t)
	t.Run("missing file", func(t *testing.T) {
		body, contentType := fileRouteMultipart(t, "", nil, 1)
		response := fileRouteRequest(fixture.router, http.MethodPost, "/api/files", fixture.token, contentType, body)
		assertFileRouteError(t, response, http.StatusBadRequest, "file is required")
	})
	t.Run("malformed multipart", func(t *testing.T) {
		body := strings.NewReader("--broken\r\nContent-Disposition: form-data; name=\"file\"; filename=\"test.bin\"\r\n\r\ntruncated")
		response := fileRouteRequest(fixture.router, http.MethodPost, "/api/files", fixture.token, "multipart/form-data; boundary=broken", body)
		assertFileRouteError(t, response, http.StatusBadRequest, "Invalid multipart request")
	})
	assertFileRouteStorageEmpty(t, fixture)
}

func TestFileRoutesUploadSizeLimits(t *testing.T) {
	for _, test := range []struct {
		name     string
		fileSize int
		padding  int
	}{
		{"file exceeds 8 MiB below body limit", services.MaxFileSize + 1, 0},
		{"body exceeds 9 MiB with small file", 1, (9 << 20) + 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newFileRouteFixture(t)
			body, contentType := fileRouteMultipart(t, "large.bin", bytes.Repeat([]byte("x"), test.fileSize), test.padding)
			if test.padding == 0 && body.Len() >= 9<<20 {
				t.Fatal("file-limit test must stay below the request-body limit")
			}
			if test.padding > 0 && body.Len() <= 9<<20 {
				t.Fatal("body-limit test must exceed 9 MiB")
			}
			response := fileRouteRequest(fixture.router, http.MethodPost, "/api/files", fixture.token, contentType, body)
			assertFileRouteError(t, response, http.StatusRequestEntityTooLarge, "file size exceeds 8 MiB limit")
			assertFileRouteStorageEmpty(t, fixture)
		})
	}
}

func TestFileRoutesMetadataDatabaseFailuresAreRedacted(t *testing.T) {
	internalErr := errors.New("postgres password=secret: internal metadata failure /private/storage")
	t.Run("save upload metadata", func(t *testing.T) {
		fixture := newFileRouteFixture(t)
		fixture.repo.uploadErr = internalErr
		body, contentType := fileRouteMultipart(t, "test.bin", []byte("plaintext"), 0)
		response := fileRouteRequest(fixture.router, http.MethodPost, "/api/files", fixture.token, contentType, body)
		assertFileRouteError(t, response, http.StatusInternalServerError, "Failed to process file")
		assertFileRouteStorageEmpty(t, fixture)
	})
	for _, route := range []struct{ name, method, suffix string }{
		{"read metadata", http.MethodGet, ""},
		{"read metadata for decrypt", http.MethodPost, "/decrypt"},
		{"save decrypt timing", http.MethodPost, "/decrypt"},
	} {
		t.Run(route.name, func(t *testing.T) {
			fixture := newFileRouteFixture(t)
			metadata := uploadFileRoute(t, fixture, "test.bin", []byte("plaintext"))
			if route.name == "save decrypt timing" {
				fixture.repo.updateErr = internalErr
			} else {
				fixture.repo.findErr = internalErr
			}
			path := fmt.Sprintf("/api/files/%d%s", metadata.ID, route.suffix)
			response := fileRouteRequest(fixture.router, route.method, path, fixture.token, "", nil)
			assertFileRouteError(t, response, http.StatusInternalServerError, "Failed to process file")
		})
	}
}

func TestFileRoutesListOwnedFiles(t *testing.T) {
	fixture := newFileRouteFixture(t)
	older := uploadFileRoute(t, fixture, "older.bin", []byte("older plaintext"))
	fixture.repo.files[2] = models.File{ID: 2, UserID: 8, OriginalName: "private-other-user.bin", AESPath: "/private/other-user"}
	newer := uploadFileRoute(t, fixture, "newer.bin", []byte("newer plaintext"))

	// Listing is metadata-only; it must not require ciphertext to exist on disk.
	for _, file := range fixture.repo.files {
		if file.UserID == 7 {
			if err := os.Remove(file.AESPath); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, path := range []string{"/api/files", "/api/files?user_id=8"} {
		response := fileRouteRequest(fixture.router, http.MethodGet, path, fixture.token, "", nil)
		files := decodeFileRouteList(t, response)
		if len(files) != 2 || !reflect.DeepEqual(files[0], newer) || !reflect.DeepEqual(files[1], older) {
			t.Fatalf("unexpected owner/order/metadata for %s: %+v", path, files)
		}
	}
}

func TestFileRoutesListEmptyAndDatabaseFailure(t *testing.T) {
	fixture := newFileRouteFixture(t)
	fixture.repo.files[1] = models.File{ID: 1, UserID: 8, OriginalName: "another-user.bin"}
	response := fileRouteRequest(fixture.router, http.MethodGet, "/api/files", fixture.token, "", nil)
	files := decodeFileRouteList(t, response)
	if files == nil || len(files) != 0 || response.Body.String() != `{"files":[]}` {
		t.Fatalf("expected a JSON empty array, got %s", response.Body.String())
	}

	fixture.repo.listErr = errors.New("postgres password=secret /private/storage")
	response = fileRouteRequest(fixture.router, http.MethodGet, "/api/files", fixture.token, "", nil)
	assertFileRouteError(t, response, http.StatusInternalServerError, "Failed to process file")
}

func decodeFileRouteList(t *testing.T, response *httptest.ResponseRecorder) []fileRouteMetadata {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("list status = %d; body: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("list must disable caching")
	}
	mediaType, _, err := mime.ParseMediaType(response.Header().Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		t.Fatalf("expected JSON list response: %v", err)
	}
	var result struct {
		Files []fileRouteMetadata `json:"files"`
	}
	decoder := json.NewDecoder(bytes.NewReader(response.Body.Bytes()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		t.Fatalf("decode allowlisted list response: %v", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		t.Fatalf("unexpected trailing response data: %v", err)
	}
	return result.Files
}

func TestSetupRouterFileRoutesCORS(t *testing.T) {
	fixture := newFileRouteFixture(t)
	fixture.router = SetupRouter(Controller{
		AuthController: controllers.NewAuthController(nil),
		FileController: fixture.controller,
	})
	plaintext := []byte("CORS download through the actual application router")
	metadata := uploadFileRoute(t, fixture, "cors.bin", plaintext)
	listRequest := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	listRequest.Header.Set("Origin", "http://localhost:5173")
	listRequest.AddCookie(&http.Cookie{Name: "access_token", Value: fixture.token})
	listResponse := httptest.NewRecorder()
	fixture.router.ServeHTTP(listResponse, listRequest)
	files := decodeFileRouteList(t, listResponse)
	if len(files) != 1 || !reflect.DeepEqual(files[0], metadata) {
		t.Fatalf("wired list returned unexpected files: %+v", files)
	}
	assertFileRouteCORS(t, listResponse.Header())

	path := fmt.Sprintf("/api/files/%d/decrypt", metadata.ID)

	preflight := httptest.NewRequest(http.MethodOptions, path, nil)
	preflight.Header.Set("Origin", "http://localhost:5173")
	preflight.Header.Set("Access-Control-Request-Method", http.MethodPost)
	preflight.Header.Set("Access-Control-Request-Headers", "Content-Type")
	response := httptest.NewRecorder()
	fixture.router.ServeHTTP(response, preflight)
	if response.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d; body: %s", response.Code, response.Body.String())
	}
	if !fileRouteHeaderContains(response.Header(), "Access-Control-Allow-Methods", "POST") ||
		!fileRouteHeaderContains(response.Header(), "Access-Control-Allow-Headers", "Content-Type") {
		t.Fatalf("preflight does not allow POST/Content-Type: %v", response.Header())
	}
	assertFileRouteCORS(t, response.Header())

	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.AddCookie(&http.Cookie{Name: "access_token", Value: fixture.token})
	response = httptest.NewRecorder()
	fixture.router.ServeHTTP(response, req)
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), plaintext) {
		t.Fatalf("wired decrypt status = %d, exact plaintext match = %t", response.Code, bytes.Equal(response.Body.Bytes(), plaintext))
	}
	assertFileRouteCORS(t, response.Header())
	for _, header := range []string{"Content-Disposition", "X-Decryption-Time-Ns", "X-Decryption-Algorithm"} {
		if !fileRouteHeaderContains(response.Header(), "Access-Control-Expose-Headers", header) || response.Header().Get(header) == "" {
			t.Errorf("download header %s must be present and exposed; headers: %v", header, response.Header())
		}
	}
}

func assertFileRouteCORS(t *testing.T, headers http.Header) {
	t.Helper()
	if headers.Get("Access-Control-Allow-Origin") != "http://localhost:5173" || headers.Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("expected credentialed localhost:5173 CORS response, got %v", headers)
	}
}

func fileRouteHeaderContains(headers http.Header, name, value string) bool {
	for _, line := range headers.Values(name) {
		for item := range strings.SplitSeq(line, ",") {
			if strings.EqualFold(strings.TrimSpace(item), value) {
				return true
			}
		}
	}
	return false
}
