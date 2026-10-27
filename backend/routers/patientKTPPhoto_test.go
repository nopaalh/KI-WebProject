package routers

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"project1/models"
	"project1/services"
	"project1/utils"

	"gorm.io/gorm"
)

func newKTPPhotoFixture(t *testing.T, format string) (*ktpRouteFixture, []byte) {
	t.Helper()
	f := newKTPRouteFixture(t)
	var photo bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var err error
	if format == "jpeg" {
		err = jpeg.Encode(&photo, img, nil)
	} else {
		err = png.Encode(&photo, img)
	}
	if err != nil {
		t.Fatal(err)
	}
	original := photo.Bytes()
	f.repo.stored.Photo = encryptedKTPFixture(t, f.key, original)
	return f, original
}

func TestReadOwnKTPPhotoIdenticalBytes(t *testing.T) {
	for _, format := range []string{"png", "jpeg"} {
		t.Run(format, func(t *testing.T) {
			f, original := newKTPPhotoFixture(t, format)
			// Photo reads need identity/ownership, not unrelated encrypted text fields.
			f.repo.stored.FullName.AES = nil
			r := requestKTP(f, "/photo?user_id=99&patient_profile_id=99", f.token)
			if r.Code != 200 || !bytes.Equal(r.Body.Bytes(), original) || r.Header().Get("Content-Type") != "image/"+format {
				t.Fatalf("photo must return identical original bytes: %d, %s", r.Code, r.Header().Get("Content-Type"))
			}
			filename := "ktp.png"
			if format == "jpeg" {
				filename = "ktp.jpg"
			}
			if r.Header().Get("Cache-Control") != "no-store" || r.Header().Get("X-Content-Type-Options") != "nosniff" ||
				r.Header().Get("Content-Disposition") != `inline; filename="`+filename+`"` {
				t.Fatalf("incorrect photo headers: %v", r.Header())
			}
			if f.users.calls != 1 {
				t.Fatalf("must check current role once, got %d lookups", f.users.calls)
			}
		})
	}
}

func TestKTPPhotoAccessAndMissingData(t *testing.T) {
	for _, scenario := range []string{"missing cookie", "invalid token", "doctor", "deleted user", "other patient", "missing profile", "missing ktp", "database error"} {
		t.Run(scenario, func(t *testing.T) {
			f, original := newKTPPhotoFixture(t, "png")
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
				// Patient 8 has a valid profile, but does not own the stored KTP/photo.
				f.profiles.stored.UserID, f.profiles.stored.ID = 8, 99
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
			r := requestKTP(f, "/photo?user_id=7&patient_profile_id=12", token)
			if r.Code != status || bytes.Equal(r.Body.Bytes(), original) || strings.Contains(r.Body.String(), "private database details") || !strings.Contains(r.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("photo error must be JSON without private data: %d, %s", r.Code, r.Body.String())
			}
			if (scenario == "doctor" || scenario == "deleted user") && f.repo.calls != 0 {
				t.Fatal("authorization failure must precede the photo query")
			}
		})
	}
}

func TestKTPPhotoRejectsInvalidStoredData(t *testing.T) {
	for _, scenario := range []string{"aes tamper", "des tamper", "rc4 tamper", "missing rc4", "mismatched plaintext", "nik mismatch", "wrong profile", "invalid image", "oversized dimensions", "oversized envelope", "wrong key"} {
		t.Run(scenario, func(t *testing.T) {
			f, original := newKTPPhotoFixture(t, "png")
			photo := &f.repo.stored.Photo
			switch scenario {
			case "aes tamper":
				photo.AES[len(photo.AES)-1] ^= 1
			case "des tamper":
				photo.DES[len(photo.DES)-1] ^= 1
			case "rc4 tamper":
				photo.RC4[len(photo.RC4)-1] ^= 1
			case "missing rc4":
				photo.RC4 = nil
			case "mismatched plaintext":
				photo.RC4 = encryptedKTPFixture(t, f.key, []byte("different bytes")).RC4
			case "nik mismatch":
				f.repo.stored.NIK = encryptedKTPFixture(t, f.key, []byte("0000000000000002"))
			case "wrong profile":
				f.repo.stored.PatientProfileID = 99
			case "invalid image":
				*photo = encryptedKTPFixture(t, f.key, []byte("not a photo"))
			case "oversized dimensions":
				// Change only the PNG header; no huge image buffer is allocated.
				large := bytes.Clone(original)
				binary.BigEndian.PutUint32(large[16:20], 5001)
				binary.BigEndian.PutUint32(large[20:24], 5001)
				binary.BigEndian.PutUint32(large[29:33], crc32.ChecksumIEEE(large[12:29]))
				*photo = encryptedKTPFixture(t, f.key, large)
			case "oversized envelope":
				photo.AES = make([]byte, services.MaxKTPPhotoSize+129)
			case "wrong key":
				key := f.key
				key[0] ^= 1
				*photo = encryptedKTPFixture(t, key, original)
			}
			r := requestKTP(f, "/photo", f.token)
			if r.Code != 500 || r.Body.String() != `{"message":"Unable to read patient KTP"}` {
				t.Fatalf("invalid photo must fail without partial bytes: %d, %s", r.Code, r.Body.String())
			}
		})
	}
}
