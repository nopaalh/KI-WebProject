package main

import (
	"os"
	"project1/config"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		panic("Error loading .env file")
	}

	config.ConnectDatabase()

	r := gin.Default()

	PORT := os.Getenv("SERVER_PORT")

	r.Run(":" + PORT)

}
