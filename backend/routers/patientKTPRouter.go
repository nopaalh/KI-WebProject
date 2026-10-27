package routers

import (
	"project1/controllers"
	"project1/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterPatientKTPRouter(api *gin.RouterGroup, controller *controllers.PatientKTPController) {
	patients := api.Group("/patients", middlewares.AuthMiddleware())
	patients.POST("/me/ktp", controller.Create)
	patients.GET("/me/ktp", controller.Get)
	patients.GET("/me/ktp/photo", controller.GetPhoto)
}
