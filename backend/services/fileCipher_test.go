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

func TestFileDownloadRejectsDamagedSelectedCiphertext(t *testing.T) {
	for _, algorithm := range []string{"aes", "des", "rc4"} {
		for _, damage := range []string{"tamper", "missing file", "symlink", "different size"} {
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
							t.Skipf("symlinks are unavailable here: %v", err)
						}
					}
				case "different size":
					// A genuine envelope of other data passes the MAC but not the recorded plaintext size.
					for _, cipher := range s.ciphers {
						if cipher.name == algorithm {
							data, err := cipher.encrypt([]byte("a longer replacement payload"), s.key)
							if err != nil {
								t.Fatal(err)
							}
							if err := os.WriteFile(*path, data, 0600); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
				_, plaintext, err := s.Decrypt(7, file.ID, algorithm)
				if !errors.Is(err, ErrFileIntegrity) || plaintext != nil || repo.updates != 0 {
					t.Fatalf("damaged selected ciphertext must fail before returning data/updating timing: %v", err)
				}
			})
		}
	}
}

func TestFileDownloadRejectsVersionOneRecordWithMissingPath(t *testing.T) {
	for _, missing := range []string{"aes", "des", "rc4"} {
		for _, algorithm := range []string{"aes", "des", "rc4"} {
			t.Run("missing "+missing+" download "+algorithm, func(t *testing.T) {
				s, repo, _ := newTestFileService(t)
				file, err := s.Upload(7, uploadHeader(t, "private.bin", []byte("original bytes")))
				if err != nil {
					t.Fatal(err)
				}
				path, _, _ := fileCipherFields(file, missing)
				*path = ""
				repo.files[file.ID] = *file
				if _, plaintext, err := s.Decrypt(7, file.ID, algorithm); !errors.Is(err, ErrFileIntegrity) || plaintext != nil || repo.updates != 0 {
					t.Fatalf("a version-1 record must keep all three paths: %v", err)
				}
			})
		}
	}
}

// Download time must reflect one cipher: only the selected envelope is decrypted, so a damaged companion
// does not block it, while that companion's own download still fails.
func TestFileDownloadDecryptsOnlySelectedCipher(t *testing.T) {
	plaintext := []byte("original bytes")
	for _, selected := range []string{"aes", "des", "rc4"} {
		t.Run(selected, func(t *testing.T) {
			s, _, _ := newTestFileService(t)
			file, err := s.Upload(7, uploadHeader(t, "private.bin", plaintext))
			if err != nil {
				t.Fatal(err)
			}
			calls := map[string]int{}
			for i := range s.ciphers {
				name, inner := s.ciphers[i].name, s.ciphers[i].decrypt
				s.ciphers[i].decrypt = func(data []byte, key [32]byte) ([]byte, error) {
					calls[name]++
					return inner(data, key)
				}
			}
			if _, got, err := s.Decrypt(7, file.ID, selected); err != nil || !bytes.Equal(got, plaintext) {
				t.Fatalf("%s download failed: %v", selected, err)
			}
			if len(calls) != 1 || calls[selected] != 1 {
				t.Fatalf("only %s should be decrypted once, got %v", selected, calls)
			}

			for _, other := range []string{"aes", "des", "rc4"} {
				if other == selected {
					continue
				}
				path, _, _ := fileCipherFields(file, other)
				data, err := os.ReadFile(*path)
				if err != nil {
					t.Fatal(err)
				}
				data[len(data)-1] ^= 1
				if err := os.WriteFile(*path, data, 0600); err != nil {
					t.Fatal(err)
				}
				if _, got, err := s.Decrypt(7, file.ID, selected); err != nil || !bytes.Equal(got, plaintext) {
					t.Fatalf("damaged %s must not block the %s download: %v", other, selected, err)
				}
				if _, got, err := s.Decrypt(7, file.ID, other); !errors.Is(err, ErrFileIntegrity) || got != nil {
					t.Fatalf("damaged %s must fail its own download: %v", other, err)
				}
			}
		})
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
