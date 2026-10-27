package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"project1/services"

	"github.com/gin-gonic/gin"
)

const maxKTPDataSize = 64 << 10

func (c *PatientKTPController) Create(ctx *gin.Context) {
	userID, ok := ktpUserID(ctx)
	if !ok {
		return
	}
	mediaType, _, err := mime.ParseMediaType(ctx.GetHeader("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		ctx.JSON(http.StatusUnsupportedMediaType, gin.H{"message": "Content-Type must be multipart/form-data"})
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, services.MaxKTPPhotoSize+maxKTPDataSize+(64<<10))
	reader, err := ctx.Request.MultipartReader()
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "Invalid multipart request"})
		return
	}
	var data, photo []byte
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			ktpUploadError(ctx, err)
			return
		}
		var target *[]byte
		var limit int64
		switch part.FormName() {
		case "data":
			if data != nil || part.FileName() != "" {
				ctx.JSON(http.StatusBadRequest, gin.H{"message": "Exactly one data field is required"})
				return
			}
			target, limit = &data, maxKTPDataSize
		case "photo":
			if photo != nil || part.FileName() == "" {
				ctx.JSON(http.StatusBadRequest, gin.H{"message": "Exactly one photo file is required"})
				return
			}
			target, limit = &photo, services.MaxKTPPhotoSize
		default:
			ctx.JSON(http.StatusBadRequest, gin.H{"message": "Only data and photo multipart fields are accepted"})
			return
		}
		value, err := io.ReadAll(io.LimitReader(part, limit+1))
		if err != nil {
			ktpUploadError(ctx, err)
			return
		}
		if int64(len(value)) > limit {
			ctx.JSON(http.StatusRequestEntityTooLarge, gin.H{"message": "Photo exceeds 8 MiB or data exceeds 64 KiB"})
			return
		}
		*target = value
		if err := part.Close(); err != nil {
			ktpUploadError(ctx, err)
			return
		}
	}
	// Consume any multipart epilogue through the same body limit before persisting.
	if _, err := io.Copy(io.Discard, ctx.Request.Body); err != nil {
		ktpUploadError(ctx, err)
		return
	}
	if len(data) == 0 || len(photo) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "Both data and photo are required"})
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var input services.CreatePatientKTPInput
	err = decoder.Decode(&input)
	if err == nil {
		var extra any
		err = decoder.Decode(&extra)
		if err == io.EOF {
			err = nil
		} else if err == nil {
			err = errors.New("multiple JSON values")
		}
	}
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "Invalid data JSON, field types, or unknown fields"})
		return
	}
	ktp, err := c.service.Create(userID, input, photo)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidKTP):
			ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		case errors.Is(err, services.ErrKTPExists):
			ctx.JSON(http.StatusConflict, gin.H{"message": "Patient KTP already exists"})
		case errors.Is(err, services.ErrProfileUnauthorized), errors.Is(err, services.ErrProfileForbidden), errors.Is(err, services.ErrProfileNotFound):
			ktpError(ctx, err)
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to create patient KTP"})
		}
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{
		"message":    "Patient KTP created",
		"ktp":        gin.H{"id": ktp.ID, "patient_profile_id": ktp.PatientProfileID},
		"encryption": gin.H{"active": []string{"aes", "des", "rc4"}, "pending": []string{}},
	})
}

func ktpUploadError(ctx *gin.Context, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		ctx.JSON(http.StatusRequestEntityTooLarge, gin.H{"message": "Multipart request exceeds the size limit"})
		return
	}
	ctx.JSON(http.StatusBadRequest, gin.H{"message": "Invalid or incomplete multipart request"})
}
