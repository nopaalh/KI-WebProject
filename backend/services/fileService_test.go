package services

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"project1/models"
	"project1/utils"

	"gorm.io/gorm"
)

type memoryFileRepository struct {
	files     map[uint]models.File
	createErr error
	listErr   error
	listCalls int
	findErr   error
	updateErr error
	updates   int
}

func (r *memoryFileRepository) UploadFile(file *models.File) error {
	if r.createErr != nil {
		return r.createErr
	}
	file.ID = uint(len(r.files) + 1)
	r.files[file.ID] = *file
	return nil
}

func (r *memoryFileRepository) ListByUserID(userID uint) ([]models.File, error) {
	r.listCalls++
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
	_, _, decryptTime := fileCipherFields(&file, algorithm)
	*decryptTime = durationNS
	r.files[fileID] = file
	r.updates++
	return nil
}

func newTestFileService(t *testing.T) (*FileService, *memoryFileRepository, string) {
	t.Helper()
	repo := &memoryFileRepository{files: make(map[uint]models.File)}
	dir := t.TempDir()
	var key [utils.AESFileKeySize]byte
	for i := range key {
		key[i] = byte(i + 1)
	}
	service, err := NewFileService(repo, dir, key)
	if err != nil {
		t.Fatal(err)
	}
	return service, repo, dir
}

func uploadHeader(t *testing.T, name string, data []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(MaxFileSize + 1); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { request.MultipartForm.RemoveAll() })
	return request.MultipartForm.File["file"][0]
}

func TestFileServicePersistsCiphertextMetadataAndTimings(t *testing.T) {
	service, repo, dir := newTestFileService(t)
	original := bytes.Repeat([]byte{0, 1, 2, 0xff, '\n'}, 100)
	file, err := service.Upload(7, uploadHeader(t, "report.bin", original))
	if err != nil {
		t.Fatal(err)
	}
	if file.ID == 0 || file.UserID != 7 || file.OriginalName != "report.bin" || file.OriginalSize != int64(len(original)) {
		t.Fatalf("incorrect metadata: %+v", file)
	}
	if file.AESEncryptTimeNS <= 0 || file.AESDecryptTimeNS <= 0 {
		t.Fatalf("missing measured durations: %+v", file)
	}
	if repo.files[file.ID] != *file {
		t.Fatal("database metadata differs from returned metadata")
	}
	if filepath.Dir(file.AESPath) != dir {
		t.Fatalf("unexpected storage path %q", file.AESPath)
	}
	encrypted, err := os.ReadFile(file.AESPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(encrypted, original) || !bytes.HasPrefix(encrypted, []byte("AES1")) {
		t.Fatal("disk does not contain the expected encrypted envelope")
	}
	info, err := os.Stat(file.AESPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("encrypted file readable by group/others: %v", info.Mode())
	}
	decoded, err := utils.DecryptAESFile(encrypted, service.key)
	if err != nil || !bytes.Equal(decoded, original) {
		t.Fatalf("disk ciphertext round trip failed: %v", err)
	}

	result, plaintext, err := service.Decrypt(7, file.ID, "aes")
	if err != nil || !bytes.Equal(plaintext, original) {
		t.Fatalf("download decryption failed: %v", err)
	}
	if repo.updates != 1 || result.AESDecryptTimeNS <= 0 || repo.files[file.ID].AESDecryptTimeNS != result.AESDecryptTimeNS {
		t.Fatal("latest decryption timing was not persisted")
	}
	if result.AESEncryptTimeNS != file.AESEncryptTimeNS {
		t.Fatal("download overwrote encryption timing")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 3 {
		t.Fatalf("expected three encrypted files on disk, got %d entries: %v", len(entries), err)
	}
}

func TestFileServiceEmptyAndDuplicateNames(t *testing.T) {
	service, _, dir := newTestFileService(t)
	first, err := service.Upload(1, uploadHeader(t, "../../same.bin", nil))
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Upload(1, uploadHeader(t, "..\\same.bin", []byte("second")))
	if err != nil {
		t.Fatal(err)
	}
	if first.AESPath == second.AESPath || first.OriginalName != "same.bin" || second.OriginalName != "same.bin" {
		t.Fatal("filename sanitization or unique storage names failed")
	}
	_, plaintext, err := service.Decrypt(1, first.ID, "aes")
	if err != nil || len(plaintext) != 0 {
		t.Fatalf("empty file decryption failed: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 6 {
		t.Fatalf("expected six stored ciphertexts: %v", err)
	}
}

func TestFileServiceRejectsInvalidUploads(t *testing.T) {
	service, repo, dir := newTestFileService(t)
	if _, err := service.Upload(0, nil); !errors.Is(err, ErrInvalidUser) {
		t.Fatalf("invalid user: %v", err)
	}
	if _, err := service.Upload(1, nil); !errors.Is(err, ErrFileRequired) {
		t.Fatalf("missing file: %v", err)
	}
	file := uploadHeader(t, "large.bin", []byte("small body"))
	file.Size = MaxFileSize + 1
	if _, err := service.Upload(1, file); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("declared oversized file: %v", err)
	}
	file = uploadHeader(t, "large.bin", make([]byte, MaxFileSize+1))
	file.Size = 1
	if _, err := service.Upload(1, file); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("actual oversized file with forged header: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 || len(repo.files) != 0 {
		t.Fatalf("rejected upload left persisted data: %v", err)
	}
}

func TestFileServiceRemovesCiphertextOnDatabaseFailure(t *testing.T) {
	service, repo, dir := newTestFileService(t)
	databaseError := errors.New("database unavailable")
	repo.createErr = databaseError
	if _, err := service.Upload(1, uploadHeader(t, "test.txt", []byte("secret"))); !errors.Is(err, databaseError) {
		t.Fatalf("expected database error, got %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("database failure left orphaned file: %v", err)
	}
}

func TestFileServiceOwnershipAndRepositoryErrors(t *testing.T) {
	service, repo, _ := newTestFileService(t)
	file, err := service.Upload(1, uploadHeader(t, "test.txt", []byte("secret")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.FindByID(0, file.ID); !errors.Is(err, ErrInvalidUser) {
		t.Fatalf("invalid user: %v", err)
	}
	for _, userID := range []uint{2, 3} {
		if _, _, err := service.Decrypt(userID, file.ID, "aes"); !errors.Is(err, ErrFileNotFound) {
			t.Fatalf("user %d read another user's file: %v", userID, err)
		}
	}
	if _, err := service.FindByID(1, file.ID+1); !errors.Is(err, ErrFileNotFound) {
		t.Fatalf("missing record: %v", err)
	}
	databaseError := errors.New("database unavailable")
	repo.findErr = databaseError
	if _, err := service.FindByID(1, file.ID); !errors.Is(err, databaseError) {
		t.Fatalf("find error not propagated: %v", err)
	}
	repo.findErr = nil
	for _, updateError := range []error{databaseError, gorm.ErrRecordNotFound} {
		repo.updateErr = updateError
		_, plaintext, err := service.Decrypt(1, file.ID, "aes")
		if plaintext != nil || err == nil {
			t.Fatal("returned plaintext despite failure to persist timing")
		}
		if updateError == gorm.ErrRecordNotFound && !errors.Is(err, ErrFileNotFound) {
			t.Fatalf("deleted record did not map to not found: %v", err)
		}
	}
}

func TestFileServiceRejectsDamagedStoredFiles(t *testing.T) {
	tests := []struct {
		name   string
		damage func(*testing.T, *FileService, *memoryFileRepository, *models.File)
	}{
		{
			name: "modified ciphertext",
			damage: func(t *testing.T, _ *FileService, _ *memoryFileRepository, file *models.File) {
				data, err := os.ReadFile(file.AESPath)
				if err != nil {
					t.Fatal(err)
				}
				data[20] ^= 1
				if err := os.WriteFile(file.AESPath, data, 0600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "wrong server key",
			damage: func(_ *testing.T, s *FileService, _ *memoryFileRepository, _ *models.File) {
				s.key[0] ^= 1
			},
		},
		{
			name: "missing disk file",
			damage: func(t *testing.T, _ *FileService, _ *memoryFileRepository, file *models.File) {
				if err := os.Remove(file.AESPath); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "historical metadata without ciphertext",
			damage: func(_ *testing.T, _ *FileService, r *memoryFileRepository, file *models.File) {
				file.AESPath = ""
				r.files[file.ID] = *file
			},
		},
		{
			name: "unexpected metadata size",
			damage: func(_ *testing.T, _ *FileService, r *memoryFileRepository, file *models.File) {
				file.OriginalSize++
				r.files[file.ID] = *file
			},
		},
		{
			name: "path outside storage",
			damage: func(_ *testing.T, _ *FileService, r *memoryFileRepository, file *models.File) {
				file.AESPath = filepath.Join(filepath.Dir(file.AESPath), "..", "aes-other.bin")
				r.files[file.ID] = *file
			},
		},
		{
			name: "oversized disk file",
			damage: func(t *testing.T, _ *FileService, _ *memoryFileRepository, file *models.File) {
				if err := os.Truncate(file.AESPath, maxEncryptedFileSize+1); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, repo, _ := newTestFileService(t)
			file, err := service.Upload(1, uploadHeader(t, "test.bin", []byte("original bytes")))
			if err != nil {
				t.Fatal(err)
			}
			tt.damage(t, service, repo, file)
			_, plaintext, err := service.Decrypt(1, file.ID, "aes")
			if !errors.Is(err, ErrFileIntegrity) || plaintext != nil {
				t.Fatalf("expected integrity error and no plaintext, got %v", err)
			}
			if repo.updates != 0 {
				t.Fatal("failed decryption changed recorded timing")
			}
		})
	}
}

func TestFileServiceListsOnlyOwnedFiles(t *testing.T) {
	service, repo, _ := newTestFileService(t)
	repo.files[1] = models.File{ID: 1, UserID: 7, OriginalName: "older.bin", AESEncryptTimeNS: 10, AESDecryptTimeNS: 20}
	repo.files[2] = models.File{ID: 2, UserID: 8, OriginalName: "other-user.bin"}
	repo.files[3] = models.File{ID: 3, UserID: 7, OriginalName: "newer.bin", AESEncryptTimeNS: 30, AESDecryptTimeNS: 40}

	files, err := service.List(7)
	if err != nil || len(files) != 2 {
		t.Fatalf("expected two owned files, got %v: %v", files, err)
	}
	for i, id := range []uint{3, 1} {
		if files[i] != repo.files[id] {
			t.Fatalf("file %d metadata/order mismatch: %+v", i, files[i])
		}
	}
	files, err = service.List(9)
	if err != nil || len(files) != 0 {
		t.Fatalf("expected an empty list for another user: %v, %v", files, err)
	}
}

func TestFileServiceListErrors(t *testing.T) {
	service, repo, _ := newTestFileService(t)
	if _, err := service.List(0); !errors.Is(err, ErrInvalidUser) {
		t.Fatalf("expected invalid user, got %v", err)
	}
	if repo.listCalls != 0 {
		t.Fatal("invalid user reached the repository")
	}
	databaseError := errors.New("database unavailable")
	repo.listErr = databaseError
	files, err := service.List(7)
	if !errors.Is(err, databaseError) || files != nil {
		t.Fatalf("expected wrapped database error without partial results: %v, %v", files, err)
	}
}

func TestNewFileServiceRejectsUnavailableStorage(t *testing.T) {
	var key [utils.AESFileKeySize]byte
	if _, err := NewFileService(nil, "", key); err == nil {
		t.Fatal("accepted empty storage directory")
	}
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileService(nil, file, key); err == nil {
		t.Fatal("accepted file as storage directory")
	}
}
