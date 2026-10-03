package routers

import (
	"project1/controllers"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	AuthController *controllers.AuthController
}

func SetupRouter(c Controller) *gin.Engine {
	r := gin.Default()

	api := r.Group("/api")
	RegisterUserRouter(api, *c.AuthController)

	return r
}
