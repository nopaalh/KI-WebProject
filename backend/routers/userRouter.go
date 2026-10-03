package routers

import (
	"github.com/gin-gonic/gin"

	"project1/controllers"
)

func RegisterUserRouter(api *gin.RouterGroup, authController controllers.AuthController) {
	user := api.Group("/users")

	user.POST("/register", authController.Register)
}
