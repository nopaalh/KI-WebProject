package routers

import (
	"github.com/gin-gonic/gin"

	"project1/controllers"
	"project1/middlewares"
)

func RegisterUserRouter(api *gin.RouterGroup, authController *controllers.AuthController) {
	user := api.Group("/users")

	user.POST("/register", authController.Register)
	user.POST("/login", authController.Login)
	user.POST("/logout", authController.Logout)
	user.GET("/me", middlewares.AuthMiddleware(), authController.Me)
}
