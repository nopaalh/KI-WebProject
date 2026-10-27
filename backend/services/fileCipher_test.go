package services

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"project1/models"
	"project1/utils"
)

func assertNoFileUploadArtifacts(t *testing.T, repo *memoryFileRepository, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 || len(repo.files) != 0 {
		t.Fatalf("failed upload left files or metadata: entries=%d, rows=%d, err=%v", len(entries), len(repo.files), err)
	}
}

func TestFileUploadStoresAndDecryptsAllCiphers(t *testing.T) {
	for _, sample := range []struct {
		name string
		data []byte
	}{
		{"empty.bin", nil},
		{"synthetic-report.pdf", []byte("%PDF-1.4\nSynthetic byte fixture, not a validated PDF\n%%EOF")},
		{"synthetic-report.docx", bytes.Repeat([]byte{0x50, 0x4b, 0, 255, '\n'}, 113)},
		{"synthetic-video.mp4", bytes.Repeat([]byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 0, 255}, 26215)},
	} {
		t.Run(sample.name, func(t *testing.T) {
			s, repo, dir := newTestFileService(t)
			file, err := s.Upload(7, uploadHeader(t, sample.name, sample.data))
			if err != nil {
				t.Fatal(err)
			}
			if file.CipherSetVersion != 1 {
				t.Fatal("new upload must explicitly require three ciphers")
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 3 {
				t.Fatalf("expected exactly three ciphertext files: %v", err)
			}
			for _, cipher := range s.ciphers {
				path, encTime, decTime := fileCipherFields(file, cipher.name)
				data, err := os.ReadFile(*path)
				if err != nil || *encTime <= 0 || *decTime <= 0 || filepath.Dir(*path) != dir {
					t.Fatalf("missing stored %s representation/timings: %v", cipher.name, err)
				}
				decoded, err := cipher.decrypt(data, s.key)
				if err != nil || !bytes.Equal(decoded, sample.data) {
					t.Fatalf("%s disk round-trip failed: %v", cipher.name, err)
				}
				result, plaintext, err := s.Decrypt(7, file.ID, cipher.name)
				if err != nil || !bytes.Equal(plaintext, sample.data) {
					t.Fatalf("%s selected download failed: %v", cipher.name, err)
				}
				_, _, duration := fileCipherFields(result, cipher.name)
				persisted := repo.files[file.ID]
				_, _, persistedDuration := fileCipherFields(&persisted, cipher.name)
				if *duration <= 0 || *duration != *persistedDuration {
					t.Fatal("selected cipher decryption timing was not persisted")
				}
			}
		})
	}
}

func TestFileUploadRollsBackOnEveryCipherFailure(t *testing.T) {
	for _, stage := range []string{"encrypt", "create", "write", "round-trip"} {
		for _, index := range []int{0, 1, 2} {
			t.Run(stage+[]string{"aes", "des", "rc4"}[index], func(t *testing.T) {
				s, repo, dir := newTestFileService(t)
				failure := errors.New("injected cipher/storage failure")
				switch stage {
				case "encrypt":
					s.ciphers[index].encrypt = func([]byte, [32]byte) ([]byte, error) { return nil, failure }
				case "round-trip":
					s.ciphers[index].decrypt = func([]byte, [32]byte) ([]byte, error) { return []byte("wrong bytes"), nil }
				case "create", "write":
					calls := 0
					s.createTemp = func(dir, pattern string) (*os.File, error) {
						current := calls
						calls++
						if current == index && stage == "create" {
							return nil, failure
						}
						file, err := os.CreateTemp(dir, pattern)
						if err == nil && current == index && stage == "write" {
							err = file.Close()
						}
						return file, err
					}
				}
				file, err := s.Upload(7, uploadHeader(t, "private.bin", []byte("private bytes")))
				if err == nil || file != nil {
					t.Fatal("partial encryption/storage must not report upload success")
				}
				assertNoFileUploadArtifacts(t, repo, dir)
			})
		}
	}
}

func TestTripleCipherDownloadRejectsIncompleteOrDamagedSet(t *testing.T) {
	for _, algorithm := range []string{"aes", "des", "rc4"} {
		for _, damage := range []string{"tamper", "missing file", "missing path", "symlink", "different plaintext"} {
			t.Run(algorithm+damage, func(t *testing.T) {
				s, repo, _ := newTestFileService(t)
				file, err := s.Upload(7, uploadHeader(t, "private.bin", []byte("original bytes")))
				if err != nil {
					t.Fatal(err)
				}
				path, _, _ := fileCipherFields(file, algorithm)
				switch damage {
				case "tamper":
					data, err := os.ReadFile(*path)
					if err != nil {
						t.Fatal(err)
					}
					data[len(data)-1] ^= 1
					if err := os.WriteFile(*path, data, 0600); err != nil {
						t.Fatal(err)
					}
				case "missing file", "symlink":
					if err := os.Remove(*path); err != nil {
						t.Fatal(err)
					}
					if damage == "symlink" {
						if err := os.Symlink(filepath.Join(t.TempDir(), "outside.bin"), *path); err != nil {
							t.Fatal(err)
						}
					}
				case "missing path":
					*path = ""
					repo.files[file.ID] = *file
				case "different plaintext":
					for _, cipher := range s.ciphers {
						if cipher.name == algorithm {
							data, err := cipher.encrypt([]byte("different data"), s.key)
							if err != nil {
								t.Fatal(err)
							}
							if err := os.WriteFile(*path, data, 0600); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
				// Even an AES download must fail if its DES or RC4 companion is invalid.
				_, plaintext, err := s.Decrypt(7, file.ID, "aes")
				if !errors.Is(err, ErrFileIntegrity) || plaintext != nil || repo.updates != 0 {
					t.Fatalf("damaged set must fail before returning data/updating timing: %v", err)
				}
			})
		}
	}
}

func TestFileCipherSelectionAndLegacyCompatibility(t *testing.T) {
	s, repo, dir := newTestFileService(t)
	plaintext := []byte("legacy AES data")
	encrypted, err := utils.EncryptAESFile(plaintext, s.key)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := os.CreateTemp(dir, "aes-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stored.Write(encrypted); err != nil {
		t.Fatal(err)
	}
	if err := stored.Close(); err != nil {
		t.Fatal(err)
	}
	repo.files[1] = models.File{ID: 1, UserID: 7, AESPath: stored.Name(), OriginalSize: int64(len(plaintext))}
	if _, decoded, err := s.Decrypt(7, 1, ""); err != nil || !bytes.Equal(decoded, plaintext) {
		t.Fatalf("legacy default AES download failed: %v", err)
	}
	for _, algorithm := range []string{"des", "rc4"} {
		if _, data, err := s.Decrypt(7, 1, algorithm); !errors.Is(err, ErrFileCipherUnavailable) || data != nil {
			t.Fatalf("legacy data must not pretend to have %s: %v", algorithm, err)
		}
	}
	for _, algorithm := range []string{"AES", "unknown", "../aes"} {
		if _, data, err := s.Decrypt(7, 1, algorithm); !errors.Is(err, ErrInvalidFileAlgorithm) || data != nil {
			t.Fatalf("invalid cipher should fail: %v", err)
		}
	}
	file, err := s.Upload(7, uploadHeader(t, "new.bin", plaintext))
	if err != nil {
		t.Fatal(err)
	}
	file.DESPath, file.RC4Path = "", ""
	repo.files[file.ID] = *file
	if _, _, err := s.Decrypt(7, file.ID, "aes"); !errors.Is(err, ErrFileIntegrity) {
		t.Fatal("a version-1 record missing both companions must not fall back to legacy")
	}
}

func TestFileUploadReportsCleanupFailure(t *testing.T) {
	s, repo, _ := newTestFileService(t)
	s.createTemp = func(dir, pattern string) (*os.File, error) {
		file, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		if err := file.Close(); err != nil {
			return nil, err
		}
		if err := os.Remove(file.Name()); err != nil {
			return nil, err
		}
		if err := os.Mkdir(file.Name(), 0700); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(file.Name(), "child"), nil, 0600); err != nil {
			return nil, err
		}
		return file, nil
	}
	file, err := s.Upload(7, uploadHeader(t, "private.bin", []byte("data")))
	if file != nil || err == nil || !strings.Contains(err.Error(), "remove uncommitted ciphertext") || len(repo.files) != 0 {
		t.Fatalf("cleanup failure must be surfaced, not reported as success: %v", err)
	}
	// The deliberately unremovable test directory is cleaned by t.TempDir, not hidden by the service.

}
