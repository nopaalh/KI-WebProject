package main

import (
	"encoding/hex"
	"os"
	"project1/config"
	"project1/controllers"
	"project1/models"
	"project1/repositories"
	"project1/routers"
	"project1/services"
	"project1/utils"

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
		&models.PatientProfile{},
	)

	if err != nil {
		panic("Failed to migrate database: " + err.Error())
	}

	userRepo := repositories.NewUserRepository(config.DB)
	authService := services.NewAuthService(userRepo)
	authController := controllers.NewAuthController(authService)

	keyBytes, err := hex.DecodeString(os.Getenv("FILE_ENCRYPTION_KEY"))
	if err != nil || len(keyBytes) != utils.AESFileKeySize {
		panic("FILE_ENCRYPTION_KEY must contain 64 hexadecimal characters (32 random bytes)")
	}
	var fileKey [utils.AESFileKeySize]byte
	copy(fileKey[:], keyBytes)
	storageDir := os.Getenv("FILE_STORAGE_DIR")
	if storageDir == "" {
		storageDir = "storage/uploads"
	}
	fileRepo := repositories.NewFileRepository(config.DB)
	fileService, err := services.NewFileService(fileRepo, storageDir, fileKey)
	if err != nil {
		panic("Failed to initialize file service: " + err.Error())
	}
	fileController := controllers.NewFileController(fileService)

	r := routers.SetupRouter(routers.Controller{
		AuthController: authController,
		FileController: fileController,
	})

	PORT := os.Getenv("SERVER_PORT")

	r.Run(":" + PORT)

}
