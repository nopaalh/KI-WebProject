package controllers

import (
	"errors"
	"mime"
	"net/http"
	"strconv"

	"project1/models"
	"project1/services"

	"github.com/gin-gonic/gin"
)

type FileController struct {
	FileService *services.FileService
}

func NewFileController(fileService *services.FileService) *FileController {
	return &FileController{FileService: fileService}
}

func (c *FileController) Upload(ctx *gin.Context) {
	userID, ok := fileUserID(ctx)
	if !ok {
		return
	}
	ctx.Header("Cache-Control", "no-store")

	// Allow multipart overhead, while the service enforces the 8 MiB file limit.
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, services.MaxFileSize+(1<<20))
	file, err := ctx.FormFile("file")
	if ctx.Request.MultipartForm != nil {
		defer ctx.Request.MultipartForm.RemoveAll()
	}
	if err != nil {
		var sizeError *http.MaxBytesError
		switch {
		case errors.As(err, &sizeError):
			fileError(ctx, services.ErrFileTooLarge)
		case errors.Is(err, http.ErrMissingFile):
			fileError(ctx, services.ErrFileRequired)
		default:
			ctx.JSON(http.StatusBadRequest, gin.H{"message": "Invalid multipart request"})
		}
		return
	}

	saved, err := c.FileService.Upload(userID, file)
	if err != nil {
		fileError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{
		"message": "File encrypted and saved successfully",
		"file":    fileResponse(saved),
	})
}

func (c *FileController) List(ctx *gin.Context) {
	userID, ok := fileUserID(ctx)
	if !ok {
		return
	}
	ctx.Header("Cache-Control", "no-store")
	files, err := c.FileService.List(userID)
	if err != nil {
		fileError(ctx, err)
		return
	}

	response := make([]gin.H, 0, len(files))
	for i := range files {
		response = append(response, fileResponse(&files[i]))
	}
	ctx.JSON(http.StatusOK, gin.H{"files": response})
}

func (c *FileController) Get(ctx *gin.Context) {
	userID, ok := fileUserID(ctx)
	if !ok {
		return
	}
	fileID, ok := fileIDParam(ctx)
	if !ok {
		return
	}
	ctx.Header("Cache-Control", "no-store")
	file, err := c.FileService.FindByID(userID, fileID)
	if err != nil {
		fileError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"file": fileResponse(file)})
}

func (c *FileController) Decrypt(ctx *gin.Context) {
	userID, ok := fileUserID(ctx)
	if !ok {
		return
	}
	fileID, ok := fileIDParam(ctx)
	if !ok {
		return
	}
	ctx.Header("Cache-Control", "no-store")
	algorithm := ctx.DefaultQuery("algorithm", "aes")
	if algorithm == "" {
		algorithm = "aes"
	}
	file, plaintext, err := c.FileService.Decrypt(userID, fileID, algorithm)
	if err != nil {
		fileError(ctx, err)
		return
	}

	ctx.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": file.OriginalName}))
	duration := file.AESDecryptTimeNS
	if algorithm == "des" {
		duration = file.DESDecryptTimeNS
	} else if algorithm == "rc4" {
		duration = file.RC4DecryptTimeNS
	}
	ctx.Header("X-Decryption-Time-Ns", strconv.FormatInt(duration, 10))
	ctx.Header("X-Decryption-Algorithm", algorithm)
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Data(http.StatusOK, "application/octet-stream", plaintext)
}

func fileResponse(file *models.File) gin.H {
	algorithms := []string{}
	for _, cipher := range []struct{ name, path string }{{"aes", file.AESPath}, {"des", file.DESPath}, {"rc4", file.RC4Path}} {
		if cipher.path != "" {
			algorithms = append(algorithms, cipher.name)
		}
	}
	return gin.H{
		"id":                  file.ID,
		"original_name":       file.OriginalName,
		"original_size":       file.OriginalSize,
		"original_type":       file.OriginalType,
		"algorithm":           services.AESFileAlgorithm,
		"algorithms":          algorithms,
		"cipher_set_version":  file.CipherSetVersion,
		"aes_encrypt_time_ns": file.AESEncryptTimeNS,
		"aes_decrypt_time_ns": file.AESDecryptTimeNS,
		"des_encrypt_time_ns": file.DESEncryptTimeNS,
		"des_decrypt_time_ns": file.DESDecryptTimeNS,
		"rc4_encrypt_time_ns": file.RC4EncryptTimeNS,
		"rc4_decrypt_time_ns": file.RC4DecryptTimeNS,
	}
}

func fileUserID(ctx *gin.Context) (uint, bool) {
	value, _ := ctx.Get("userID")
	userID, ok := value.(uint)
	if !ok || userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return 0, false
	}
	return userID, true
}

func fileIDParam(ctx *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, strconv.IntSize)
	if err != nil || id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "Invalid file ID"})
		return 0, false
	}
	return uint(id), true
}

func fileError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrInvalidUser):
		ctx.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
	case errors.Is(err, services.ErrInvalidFileAlgorithm):
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	case errors.Is(err, services.ErrFileCipherUnavailable):
		ctx.JSON(http.StatusConflict, gin.H{"message": err.Error()})
	case errors.Is(err, services.ErrFileRequired):
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	case errors.Is(err, services.ErrFileTooLarge):
		ctx.JSON(http.StatusRequestEntityTooLarge, gin.H{"message": err.Error()})
	case errors.Is(err, services.ErrFileNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
	default:
		ctx.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to process file"})
	}
}
