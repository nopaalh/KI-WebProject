package controllers

import (
	"errors"
	"net/http"

	"project1/services"

	"github.com/gin-gonic/gin"
)

type PatientKTPController struct {
	service *services.PatientKTPService
}

func NewPatientKTPController(service *services.PatientKTPService) *PatientKTPController {
	return &PatientKTPController{service: service}
}

func ktpUserID(ctx *gin.Context) (uint, bool) {
	ctx.Header("Cache-Control", "no-store")
	ctx.Header("X-Content-Type-Options", "nosniff")
	value, _ := ctx.Get("userID")
	userID, ok := value.(uint)
	if !ok || userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return 0, false
	}
	return userID, true
}

func (c *PatientKTPController) Get(ctx *gin.Context) {
	userID, ok := ktpUserID(ctx)
	if !ok {
		return
	}
	ktp, err := c.service.Get(userID)
	if err != nil {
		ktpError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"ktp": ktp})
}

func ktpError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrProfileUnauthorized):
		ctx.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
	case errors.Is(err, services.ErrProfileForbidden):
		ctx.JSON(http.StatusForbidden, gin.H{"message": "Only patients can access their own KTP"})
	case errors.Is(err, services.ErrProfileNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"message": "Patient profile not found"})
	case errors.Is(err, services.ErrKTPNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"message": "Patient KTP not found"})
	default:
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to read patient KTP"})
	}
}
