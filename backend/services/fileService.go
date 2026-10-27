package services

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"project1/models"
	"project1/utils"

	"gorm.io/gorm"
)

const (
	MaxFileSize      = 8 << 20
	AESFileAlgorithm = "AES-128-CBC-HMAC-SHA256"
	DESFileAlgorithm = "DES-CTR-HMAC-SHA256"
	RC4FileAlgorithm = "RC4-drop1536-HMAC-SHA256"
	// AES has the largest envelope: magic, IV, a full padding block, and MAC.
	maxEncryptedFileSize = MaxFileSize + 4 + 16 + 16 + 32
)

var (
	ErrInvalidUser           = errors.New("invalid user ID")
	ErrFileRequired          = errors.New("file is required")
	ErrFileTooLarge          = errors.New("file size exceeds 8 MiB limit")
	ErrFileNotFound          = errors.New("file not found")
	ErrFileIntegrity         = errors.New("encrypted file is unavailable or failed integrity verification")
	ErrInvalidFileAlgorithm  = errors.New("algorithm must be aes, des, or rc4")
	ErrFileCipherUnavailable = errors.New("requested cipher is unavailable for this legacy file; upload the original again")
)

type fileRepository interface {
	UploadFile(*models.File) error
	ListByUserID(userID uint) ([]models.File, error)
	FindByIDAndUserID(fileID, userID uint) (*models.File, error)
	UpdateDecryptTime(fileID, userID uint, algorithm string, durationNS int64) error
}

type fileCipher struct {
	name    string
	encrypt func([]byte, [32]byte) ([]byte, error)
	decrypt func([]byte, [32]byte) ([]byte, error)
}

type FileService struct {
	fileRepo   fileRepository
	storageDir string
	key        [utils.AESFileKeySize]byte
	ciphers    []fileCipher
	createTemp func(string, string) (*os.File, error)
}

func NewFileService(fRepo fileRepository, storageDir string, key [utils.AESFileKeySize]byte) (*FileService, error) {
	if storageDir == "" {
		return nil, errors.New("file storage directory is required")
	}
	absoluteDir, err := filepath.Abs(storageDir)
	if err != nil {
		return nil, fmt.Errorf("resolve file storage directory: %w", err)
	}
	if err := os.MkdirAll(absoluteDir, 0700); err != nil {
		return nil, fmt.Errorf("create file storage directory: %w", err)
	}
	return &FileService{
		fileRepo: fRepo, storageDir: absoluteDir, key: key, createTemp: os.CreateTemp,
		ciphers: []fileCipher{
			{"aes", utils.EncryptAESFile, utils.DecryptAESFile},
			{"des", utils.EncryptDESFile, utils.DecryptDESFile},
			{"rc4", utils.EncryptRC4File, utils.DecryptRC4File},
		},
	}, nil
}

func (s *FileService) Upload(userID uint, file *multipart.FileHeader) (saved *models.File, err error) {
	if userID == 0 {
		return nil, ErrInvalidUser
	}
	if file == nil {
		return nil, ErrFileRequired
	}
	if file.Size > MaxFileSize {
		return nil, ErrFileTooLarge
	}
	source, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open uploaded file: %w", err)
	}
	defer source.Close()
	plaintext, err := io.ReadAll(io.LimitReader(source, MaxFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("read uploaded file: %w", err)
	}
	if len(plaintext) > MaxFileSize {
		return nil, ErrFileTooLarge
	}
	newFile := &models.File{
		UserID: userID, OriginalSize: int64(len(plaintext)), CipherSetVersion: 1,
		OriginalName: filepath.Base(strings.ReplaceAll(file.Filename, "\\", "/")),
		OriginalType: file.Header.Get("Content-Type"),
	}
	var paths []string
	persisted := false
	defer func() {
		if !persisted {
			for _, path := range paths {
				if cleanupErr := os.Remove(path); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
					err = errors.Join(err, fmt.Errorf("remove uncommitted ciphertext: %w", cleanupErr))
				}
			}
		}
	}()

	for _, cipher := range s.ciphers {
		path, encryptTime, decryptTime := fileCipherFields(newFile, cipher.name)
		start := time.Now()
		encrypted, encryptErr := cipher.encrypt(plaintext, s.key)
		*encryptTime = time.Since(start).Nanoseconds()
		if encryptErr != nil {
			return nil, fmt.Errorf("encrypt uploaded file with %s: %w", cipher.name, encryptErr)
		}
		stored, createErr := s.createTemp(s.storageDir, cipher.name+"-*.bin")
		if createErr != nil {
			return nil, fmt.Errorf("create %s ciphertext: %w", cipher.name, createErr)
		}
		*path = stored.Name()
		paths = append(paths, *path)
		_, writeErr := stored.Write(encrypted)
		if writeErr == nil {
			writeErr = stored.Sync()
		}
		closeErr := stored.Close()
		if writeErr != nil || closeErr != nil {
			return nil, fmt.Errorf("persist %s ciphertext: %w", cipher.name, errors.Join(writeErr, closeErr))
		}

		onDisk, readErr := s.readCiphertext(*path, cipher.name)
		if readErr != nil {
			return nil, readErr
		}
		start = time.Now()
		decoded, decryptErr := cipher.decrypt(onDisk, s.key)
		*decryptTime = time.Since(start).Nanoseconds()
		if decryptErr != nil || !bytes.Equal(decoded, plaintext) {
			return nil, ErrFileIntegrity
		}
	}
	// No metadata is published until all three writes and disk round trips succeed.
	if err := s.fileRepo.UploadFile(newFile); err != nil {
		return nil, fmt.Errorf("save file metadata: %w", err)
	}
	persisted = true
	return newFile, nil
}

func (s *FileService) List(userID uint) ([]models.File, error) {
	if userID == 0 {
		return nil, ErrInvalidUser
	}
	files, err := s.fileRepo.ListByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	return files, nil
}

func (s *FileService) FindByID(userID, fileID uint) (*models.File, error) {
	if userID == 0 {
		return nil, ErrInvalidUser
	}
	file, err := s.fileRepo.FindByIDAndUserID(fileID, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrFileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find file: %w", err)
	}
	return file, nil
}

func (s *FileService) Decrypt(userID, fileID uint, algorithm string) (*models.File, []byte, error) {
	if algorithm == "" {
		algorithm = "aes"
	}
	if algorithm != "aes" && algorithm != "des" && algorithm != "rc4" {
		return nil, nil, ErrInvalidFileAlgorithm
	}
	file, err := s.FindByID(userID, fileID)
	if err != nil {
		return nil, nil, err
	}
	if file.OriginalSize < 0 || file.OriginalSize > MaxFileSize {
		return nil, nil, ErrFileIntegrity
	}
	// An explicit version prevents missing paths in new uploads from enabling legacy fallback.
	switch file.CipherSetVersion {
	case 0:
		if file.DESPath != "" || file.RC4Path != "" {
			return nil, nil, ErrFileIntegrity
		}
		if algorithm != "aes" {
			return nil, nil, ErrFileCipherUnavailable
		}
	case 1:
		// All three representations must be recorded, including for the default AES download, but only the
		// selected one is read and decrypted so the request time reflects that single cipher.
		if file.AESPath == "" || file.DESPath == "" || file.RC4Path == "" {
			return nil, nil, ErrFileIntegrity
		}
	default:
		return nil, nil, ErrFileIntegrity
	}
	var selected *fileCipher
	for i := range s.ciphers {
		if s.ciphers[i].name == algorithm {
			selected = &s.ciphers[i]
		}
	}
	path, _, _ := fileCipherFields(file, algorithm)
	encrypted, err := s.readCiphertext(*path, algorithm)
	if err != nil {
		return nil, nil, err
	}
	start := time.Now()
	plaintext, err := selected.decrypt(encrypted, s.key)
	selectedDuration := time.Since(start).Nanoseconds()
	if err != nil || int64(len(plaintext)) != file.OriginalSize {
		return nil, nil, ErrFileIntegrity
	}
	if err := s.fileRepo.UpdateDecryptTime(file.ID, userID, algorithm, selectedDuration); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrFileNotFound
		}
		return nil, nil, fmt.Errorf("save decryption duration: %w", err)
	}
	_, _, decryptTime := fileCipherFields(file, algorithm)
	*decryptTime = selectedDuration
	return file, plaintext, nil
}

func (s *FileService) readCiphertext(path, algorithm string) ([]byte, error) {
	relative, err := filepath.Rel(s.storageDir, path)
	if err != nil || relative == "." || filepath.Base(relative) != relative || !strings.HasPrefix(relative, algorithm+"-") {
		return nil, ErrFileIntegrity
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, ErrFileIntegrity
	}
	stored, err := os.Open(path)
	if err != nil {
		return nil, ErrFileIntegrity
	}
	defer stored.Close()
	opened, err := stored.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() {
		return nil, ErrFileIntegrity
	}
	encrypted, err := io.ReadAll(io.LimitReader(stored, maxEncryptedFileSize+1))
	if err != nil || len(encrypted) > maxEncryptedFileSize {
		return nil, ErrFileIntegrity
	}
	return encrypted, nil
}

func fileCipherFields(file *models.File, algorithm string) (path *string, encryptTime, decryptTime *int64) {
	switch algorithm {
	case "aes":
		return &file.AESPath, &file.AESEncryptTimeNS, &file.AESDecryptTimeNS
	case "des":
		return &file.DESPath, &file.DESEncryptTimeNS, &file.DESDecryptTimeNS
	case "rc4":
		return &file.RC4Path, &file.RC4EncryptTimeNS, &file.RC4DecryptTimeNS
	default:
		panic("unsupported internal file cipher")
	}
}
