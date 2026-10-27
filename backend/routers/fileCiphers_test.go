package routers

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"testing"

	"project1/services"
)

func TestFileRoutesDecryptEachCipherAndPreserveOwnership(t *testing.T) {
	fixture := newFileRouteFixture(t)
	plaintext := bytes.Repeat([]byte{0, 255, '\n', 128, 1}, 257)
	metadata := uploadFileRoute(t, fixture, "synthetic-report.docx", plaintext)
	if metadata.CipherSetVersion != 1 || !reflect.DeepEqual(metadata.Algorithms, []string{"aes", "des", "rc4"}) ||
		metadata.DESEncryptTimeNS <= 0 || metadata.DESDecryptTimeNS <= 0 || metadata.RC4EncryptTimeNS <= 0 || metadata.RC4DecryptTimeNS <= 0 {
		t.Fatalf("upload must report three actually stored representations: %+v", metadata)
	}
	for _, algorithm := range []string{"aes", "des", "rc4"} {
		path := fmt.Sprintf("/api/files/%d/decrypt?algorithm=%s", metadata.ID, algorithm)
		response := fileRouteRequest(fixture.router, http.MethodPost, path, fixture.token, "", nil)
		if response.Code != 200 || !bytes.Equal(response.Body.Bytes(), plaintext) || response.Header().Get("X-Decryption-Algorithm") != algorithm {
			t.Fatalf("%s download failed: %d", algorithm, response.Code)
		}
		stored := fixture.repo.files[metadata.ID]
		duration := stored.AESDecryptTimeNS
		if algorithm == "des" {
			duration = stored.DESDecryptTimeNS
		} else if algorithm == "rc4" {
			duration = stored.RC4DecryptTimeNS
		}
		if response.Header().Get("X-Decryption-Time-Ns") != strconv.FormatInt(duration, 10) || duration <= 0 {
			t.Fatal("header must use the selected cipher's persisted duration")
		}
		response = fileRouteRequest(fixture.router, http.MethodPost, path, fileRouteToken(t, 8), "", nil)
		assertFileRouteError(t, response, 404, "file not found")
	}
	invalid := fmt.Sprintf("/api/files/%d/decrypt?algorithm=invalid", metadata.ID)
	response := fileRouteRequest(fixture.router, http.MethodPost, invalid, fixture.token, "", nil)
	assertFileRouteError(t, response, 400, services.ErrInvalidFileAlgorithm.Error())

	stored := fixture.repo.files[metadata.ID]
	if err := os.Remove(stored.RC4Path); err != nil {
		t.Fatal(err)
	}
	// Only the selected ciphertext is read, so losing the RC4 copy fails the RC4 download but not the AES one.
	path := fmt.Sprintf("/api/files/%d/decrypt?algorithm=aes", metadata.ID)
	response = fileRouteRequest(fixture.router, http.MethodPost, path, fixture.token, "", nil)
	if response.Code != 200 || !bytes.Equal(response.Body.Bytes(), plaintext) {
		t.Fatalf("AES download must not depend on the RC4 copy: %d", response.Code)
	}
	path = fmt.Sprintf("/api/files/%d/decrypt?algorithm=rc4", metadata.ID)
	response = fileRouteRequest(fixture.router, http.MethodPost, path, fixture.token, "", nil)
	assertFileRouteError(t, response, 500, "Failed to process file")
}

func TestFileRoutesExposeLegacyCipherAvailability(t *testing.T) {
	fixture := newFileRouteFixture(t)
	metadata := uploadFileRoute(t, fixture, "legacy-fixture.bin", []byte("legacy bytes"))
	stored := fixture.repo.files[metadata.ID]
	// Simulate a pre-upgrade row; the separately verified service test creates a real AES-only fixture.
	stored.CipherSetVersion, stored.DESPath, stored.RC4Path = 0, "", ""
	fixture.repo.files[metadata.ID] = stored
	response := fileRouteRequest(fixture.router, http.MethodGet, fmt.Sprintf("/api/files/%d", metadata.ID), fixture.token, "", nil)
	got := decodeFileRouteResponse(t, response, 200).File
	if got.CipherSetVersion != 0 || !reflect.DeepEqual(got.Algorithms, []string{"aes"}) {
		t.Fatalf("legacy metadata must not claim all three ciphers: %+v", got)
	}
	response = fileRouteRequest(fixture.router, http.MethodPost, fmt.Sprintf("/api/files/%d/decrypt?algorithm=des", metadata.ID), fixture.token, "", nil)
	assertFileRouteError(t, response, 409, services.ErrFileCipherUnavailable.Error())
}
