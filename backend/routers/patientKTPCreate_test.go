package routers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"project1/models"
	"project1/services"

	"gorm.io/gorm"
)

const validKTPData = `{"full_name":"Pasien Sintetis","birth_place":"Kota Contoh","birth_date":"2005-01-01","gender":"Laki-laki","blood_type":"O","address":"Jalan Contoh 1","rt":"001","rw":"002","village":"Desa Contoh","district":"Kecamatan Contoh","religion":"Islam","marital_status":"Belum Kawin","occupation":"Pelajar","citizenship":"WNI"}`

type ktpUploadPart struct {
	name, filename string
	data           []byte
}

func ktpMultipart(t *testing.T, parts []ktpUploadPart) (*bytes.Buffer, string) {
	t.Helper()
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	for _, part := range parts {
		var target io.Writer
		var err error
		if part.filename != "" {
			target, err = writer.CreateFormFile(part.name, part.filename)
		} else {
			target, err = writer.CreateFormField(part.name)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := target.Write(part.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body, writer.FormDataContentType()
}

func serveKTPUpload(f *ktpRouteFixture, token string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/patients/me/ktp?patient_profile_id=99&user_id=99", body)
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	}
	response := httptest.NewRecorder()
	f.router.ServeHTTP(response, req)
	return response
}

func uploadKTP(t *testing.T, f *ktpRouteFixture, token, data string, photo []byte) *httptest.ResponseRecorder {
	t.Helper()
	body, contentType := ktpMultipart(t, []ktpUploadPart{{"data", "", []byte(data)}, {"photo", "ktp.png", photo}})
	return serveKTPUpload(f, token, body, contentType)
}

func TestCreateKTPAndReadDataAndPhoto(t *testing.T) {
	for _, format := range []string{"png", "jpeg"} {
		t.Run(format, func(t *testing.T) {
			f, photo := newKTPPhotoFixture(t, format)
			f.repo.stored = nil
			r := uploadKTP(t, f, f.token, validKTPData, photo)
			if r.Code != http.StatusCreated || r.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("create KTP: %d, %s", r.Code, r.Body.String())
			}
			if f.repo.stored == nil || f.repo.stored.PatientProfileID != 12 || f.users.calls != 1 {
				t.Fatal("KTP must use the logged-in patient's profile, with one role lookup")
			}
			if !strings.Contains(r.Body.String(), `"active":["aes","des","rc4"]`) || strings.Contains(r.Body.String(), "Pasien Sintetis") || strings.Contains(r.Body.String(), "0000000000000001") {
				t.Fatal("create must report all ciphers without exposing identity fields")
			}
			read := requestKTP(f, "", f.token)
			if read.Code != 200 {
				t.Fatalf("read created KTP: %d, %s", read.Code, read.Body.String())
			}
			var decoded struct {
				KTP map[string]any `json:"ktp"`
			}
			var original map[string]any
			if err := json.Unmarshal(read.Body.Bytes(), &decoded); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(validKTPData), &original); err != nil {
				t.Fatal(err)
			}
			for key, value := range original {
				if decoded.KTP[key] != value {
					t.Fatalf("field %s did not round-trip: %v", key, decoded.KTP[key])
				}
			}
			if decoded.KTP["nik"] != "0000000000000001" {
				t.Fatal("NIK must come from the patient's profile")
			}
			image := requestKTP(f, "/photo", f.token)
			if image.Code != 200 || !bytes.Equal(image.Body.Bytes(), photo) {
				t.Fatal("uploaded photo must round-trip identically through all three ciphers")
			}
			stored := f.repo.stored
			if duplicate := uploadKTP(t, f, f.token, validKTPData, photo); duplicate.Code != 409 || f.repo.stored != stored {
				t.Fatalf("duplicate must not overwrite KTP: %d", duplicate.Code)
			}
		})
	}
}

func TestCreateKTPRejectsInvalidDataWithoutSaving(t *testing.T) {
	for _, data := range []string{
		`{`, `null`, validKTPData + `{}`,
		strings.Replace(validKTPData, "{", `{"patient_profile_id":99,`, 1),
		strings.Replace(validKTPData, "{", `{"nik":"0000000000000002",`, 1),
		strings.Replace(validKTPData, "Pasien Sintetis", " ", 1),
		strings.Replace(validKTPData, "2005-01-01", "2005-02-30", 1),
		strings.Replace(validKTPData, "2005-01-01", "2999-01-01", 1),
		strings.Replace(validKTPData, "Laki-laki", "unknown", 1),
		strings.Replace(validKTPData, `"blood_type":"O"`, `"blood_type":"Z"`, 1),
		strings.Replace(validKTPData, "WNI", "other", 1),
		strings.Replace(validKTPData, "Belum Kawin", "unknown", 1),
		strings.Replace(validKTPData, `"rt":"001"`, `"rt":"0001"`, 1),
		strings.Replace(validKTPData, `"rw":"002"`, `"rw":2`, 1),
	} {
		t.Run(data, func(t *testing.T) {
			f, photo := newKTPPhotoFixture(t, "png")
			f.repo.stored = nil
			if r := uploadKTP(t, f, f.token, data, photo); r.Code != 400 || f.repo.stored != nil {
				t.Fatalf("invalid KTP must not persist: %d, %s", r.Code, r.Body.String())
			}
		})
	}
}

func TestCreateKTPRejectsMalformedMultipartAndPhotos(t *testing.T) {
	f, photo := newKTPPhotoFixture(t, "png")
	f.repo.stored = nil
	validParts := []ktpUploadPart{{"data", "", []byte(validKTPData)}, {"photo", "ktp.png", photo}}
	for _, parts := range [][]ktpUploadPart{
		validParts[:1], validParts[1:],
		{validParts[0], validParts[0], validParts[1]},
		{validParts[0], validParts[1], validParts[1]},
		{validParts[0], validParts[1], {"user_id", "", []byte("99")}},
		{{"data", "data.json", []byte(validKTPData)}, validParts[1]},
		{validParts[0], {"photo", "", photo}},
		{validParts[0], {"photo", "ktp.png", []byte("not an image")}},
		{validParts[0], {"photo", "ktp.png", photo[:len(photo)-10]}},
	} {
		body, contentType := ktpMultipart(t, parts)
		if r := serveKTPUpload(f, f.token, body, contentType); r.Code != 400 || f.repo.stored != nil {
			t.Fatalf("invalid multipart/photo must not persist: %d, %s", r.Code, r.Body.String())
		}
	}
	if r := serveKTPUpload(f, f.token, strings.NewReader(validKTPData), "application/json"); r.Code != 415 {
		t.Fatalf("expected 415, got %d", r.Code)
	}
	if r := serveKTPUpload(f, f.token, strings.NewReader("broken"), "multipart/form-data"); r.Code != 400 {
		t.Fatalf("missing multipart boundary must return 400, got %d", r.Code)
	}
}

func TestCreateKTPUploadLimits(t *testing.T) {
	f, photo := newKTPPhotoFixture(t, "png")
	f.repo.stored = nil
	if r := uploadKTP(t, f, f.token, validKTPData, make([]byte, services.MaxKTPPhotoSize+1)); r.Code != 413 {
		t.Fatalf("oversized photo: expected 413, got %d", r.Code)
	}
	if r := uploadKTP(t, f, f.token, strings.Repeat("x", (64<<10)+1), photo); r.Code != 413 {
		t.Fatalf("oversized data: expected 413, got %d", r.Code)
	}
	body, contentType := ktpMultipart(t, []ktpUploadPart{{"data", "", []byte(validKTPData)}, {"photo", "ktp.png", photo}})
	withEpilogue := io.MultiReader(body, strings.NewReader(strings.Repeat("x", services.MaxKTPPhotoSize+(128<<10))))
	if r := serveKTPUpload(f, f.token, withEpilogue, contentType); r.Code != 413 || f.repo.stored != nil {
		t.Fatalf("oversized whole body must not persist: %d", r.Code)
	}
}

func TestCreateKTPAccessAndDatabaseFailure(t *testing.T) {
	for _, scenario := range []string{"no cookie", "invalid token", "doctor", "deleted user", "missing profile", "database error"} {
		t.Run(scenario, func(t *testing.T) {
			f, photo := newKTPPhotoFixture(t, "png")
			f.repo.stored = nil
			token, status := f.token, 500
			switch scenario {
			case "no cookie":
				token, status = "", 401
			case "invalid token":
				token, status = "invalid", 401
			case "doctor":
				f.users.role, status = models.RoleDoctor, 403
			case "deleted user":
				f.users.err, status = gorm.ErrRecordNotFound, 401
			case "missing profile":
				f.profiles.stored, status = nil, 404
			case "database error":
				f.repo.err = errors.New("private database details")
			}
			r := uploadKTP(t, f, token, validKTPData, photo)
			if r.Code != status || f.repo.stored != nil || strings.Contains(r.Body.String(), "private database details") {
				t.Fatalf("failed create must not save or leak details: %d, %s", r.Code, r.Body.String())
			}
		})
	}
}
