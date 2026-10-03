package main

import (
	"os"
	"project1/config"
	"project1/controllers"
	"project1/models"
	"project1/repositories"
	"project1/routers"
	"project1/services"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		panic("Error loading .env file")
	}

	config.ConnectDatabase()
	err = config.DB.AutoMigrate(
		&models.User{},
		&models.File{},
	)

	if err != nil {
		panic("Failed to migrate database: " + err.Error())
	}

	userRepo := repositories.NewUserRepository(config.DB)
	authService := services.NewAuthService(userRepo)
	authController := controllers.NewAuthController(authService)

	r := routers.SetupRouter(routers.Controller{
		AuthController: authController,
	})

	PORT := os.Getenv("SERVER_PORT")

	r.Run(":" + PORT)

}
