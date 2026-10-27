package routers

import (
	"time"

	"project1/controllers"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Controller struct {
	AuthController           *controllers.AuthController
	FileController           *controllers.FileController
	PatientProfileController *controllers.PatientProfileController
	PatientKTPController     *controllers.PatientKTPController
	DoctorController         *controllers.DoctorController
}

func SetupRouter(c Controller) *gin.Engine {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:5173",
		},
		AllowMethods: []string{
			"GET",
			"POST",
			"PUT",
			"PATCH",
			"DELETE",
			"OPTIONS",
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Authorization",
		},
		ExposeHeaders: []string{
			"Content-Length",
			"Content-Disposition",
			"X-Decryption-Time-Ns",
			"X-Decryption-Algorithm",
		},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	api := r.Group("/api")

	RegisterUserRouter(api, c.AuthController)
	RegisterFileRouter(api, c.FileController)
	if c.PatientProfileController != nil {
		RegisterPatientRouter(api, c.PatientProfileController)
	}

	if c.PatientKTPController != nil {
		RegisterPatientKTPRouter(api, c.PatientKTPController)
	}

	if c.DoctorController != nil {
		RegisterDoctorRouter(api, c.DoctorController, c.FileController)
	}

	return r
}
