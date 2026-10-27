package routers

import (
	"project1/controllers"
	"project1/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterPatientRouter(api *gin.RouterGroup, profileController *controllers.PatientProfileController) {
	patients := api.Group("/patients", middlewares.AuthMiddleware())
	patients.POST("/me/profile", profileController.Create)
	patients.GET("/me/profile", profileController.Get)
}
