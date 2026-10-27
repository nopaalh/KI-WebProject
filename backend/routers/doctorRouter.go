package routers

import (
	"project1/controllers"
	"project1/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterDoctorRouter(api *gin.RouterGroup, doctorController *controllers.DoctorController, fileController *controllers.FileController) {
	doctor := api.Group("/doctor", middlewares.AuthMiddleware())
	doctor.GET("/patients", doctorController.ListPatients)
	if fileController != nil {
		doctor.POST("/patients/:patient_profile_id/files", fileController.UploadForPatient)
		doctor.GET("/patients/:patient_profile_id/files", fileController.ListForPatient)
	}
}
