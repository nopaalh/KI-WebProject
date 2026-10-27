package controllers

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"project1/services"

	"github.com/gin-gonic/gin"
)

type PatientProfileController struct {
	service *services.PatientProfileService
}

func NewPatientProfileController(service *services.PatientProfileService) *PatientProfileController {
	return &PatientProfileController{service: service}
}

func (c *PatientProfileController) Get(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	value, _ := ctx.Get("userID")
	userID, ok := value.(uint)
	if !ok || userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}
	profile, err := c.service.Get(userID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrProfileUnauthorized):
			ctx.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		case errors.Is(err, services.ErrProfileForbidden):
			ctx.JSON(http.StatusForbidden, gin.H{"message": err.Error()})
		case errors.Is(err, services.ErrProfileNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"message": "Patient profile not found"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to read patient profile"})
		}
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"profile": profile})
}

func (c *PatientProfileController) Create(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	userID, ok := ctx.Get("userID")
	id, valid := userID.(uint)
	if !ok || !valid || id == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}
	mediaType, _, err := mime.ParseMediaType(ctx.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		ctx.JSON(http.StatusUnsupportedMediaType, gin.H{"message": "Content-Type must be application/json"})
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 4096)
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	var input services.CreatePatientProfileInput
	err = decoder.Decode(&input)
	if err == nil {
		var extra any
		err = decoder.Decode(&extra)
		if err == io.EOF {
			err = nil
		} else if err == nil {
			err = errors.New("request must contain one JSON object")
		}
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			ctx.JSON(http.StatusRequestEntityTooLarge, gin.H{"message": "Request body exceeds 4 KiB"})
		} else {
			ctx.JSON(http.StatusBadRequest, gin.H{"message": "Invalid JSON, unknown fields, or invalid field types"})
		}
		return
	}
	profile, err := c.service.Create(id, input)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrProfileUnauthorized):
			ctx.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		case errors.Is(err, services.ErrProfileForbidden):
			ctx.JSON(http.StatusForbidden, gin.H{"message": err.Error()})
		case errors.Is(err, services.ErrInvalidProfile):
			ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		case errors.Is(err, services.ErrProfileExists):
			ctx.JSON(http.StatusConflict, gin.H{"message": "Patient profile already exists"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to create patient profile"})
		}
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{
		"message": "Patient profile created",
		"profile": gin.H{
			"id": profile.ID, "user_id": profile.UserID,
			"weight_kg": profile.WeightKG, "height_cm": profile.HeightCM, "age": profile.Age,
		},
		"encryption": gin.H{"active": []string{"aes", "des", "rc4"}, "pending": []string{}},
	})
}
