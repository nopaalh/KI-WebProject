package controllers

import (
	"errors"
	"net/http"

	"project1/repositories"
	"project1/services"

	"github.com/gin-gonic/gin"
)

type DoctorController struct {
	DoctorService *services.DoctorService
}

func NewDoctorController(doctorService *services.DoctorService) *DoctorController {
	return &DoctorController{DoctorService: doctorService}
}

func (c *DoctorController) ListPatients(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	value, _ := ctx.Get("userID")
	userID, ok := value.(uint)
	if !ok || userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}
	patients, err := c.DoctorService.ListPatients(userID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidUser):
			ctx.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		case errors.Is(err, services.ErrAccessForbidden):
			ctx.JSON(http.StatusForbidden, gin.H{"message": "Forbidden"})
		default:
			ctx.Error(err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to list patients"})
		}
		return
	}
	if patients == nil {
		patients = []repositories.PatientSummary{}
	}
	ctx.JSON(http.StatusOK, gin.H{"patients": patients})
}
