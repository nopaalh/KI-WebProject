package main

import (
	"os"
	"project1/config"
	"project1/models"

	"github.com/gin-gonic/gin"
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
	r := gin.Default()

	PORT := os.Getenv("SERVER_PORT")

	r.Run(":" + PORT)

}
