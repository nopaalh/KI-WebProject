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
		&models.PatientKTP{},
	)

	if err != nil {
		panic("Failed to migrate database: " + err.Error())
	}

	keyBytes, err := hex.DecodeString(os.Getenv("FILE_ENCRYPTION_KEY"))
	if err != nil || len(keyBytes) != utils.AESFileKeySize {
		panic("FILE_ENCRYPTION_KEY must contain 64 hexadecimal characters (32 random bytes)")
	}
	var fileKey [utils.AESFileKeySize]byte
	copy(fileKey[:], keyBytes)

	userRepo := repositories.NewUserRepository(config.DB)
	profileRepo := repositories.NewPatientProfileRepository(config.DB)
	authService := services.NewAuthService(userRepo, fileKey)
	authController := controllers.NewAuthController(authService)

	storageDir := os.Getenv("FILE_STORAGE_DIR")
	if storageDir == "" {
		storageDir = "storage/uploads"
	}
	fileRepo := repositories.NewFileRepository(config.DB)
	fileService, err := services.NewFileService(fileRepo, userRepo, profileRepo, storageDir, fileKey)
	if err != nil {
		panic("Failed to initialize file service: " + err.Error())
	}
	fileController := controllers.NewFileController(fileService)

	profileService := services.NewPatientProfileService(profileRepo, userRepo, fileKey)
	profileController := controllers.NewPatientProfileController(profileService)

	ktpRepo := repositories.NewPatientKTPRepository(config.DB)
	ktpService := services.NewPatientKTPService(ktpRepo, profileService, fileKey)
	ktpController := controllers.NewPatientKTPController(ktpService)

	doctorRepo := repositories.NewDoctorRepository(config.DB)
	doctorService := services.NewDoctorService(userRepo, doctorRepo, fileKey)
	doctorController := controllers.NewDoctorController(doctorService)

	r := routers.SetupRouter(routers.Controller{
		AuthController:           authController,
		FileController:           fileController,
		PatientProfileController: profileController,
		PatientKTPController:     ktpController,
		DoctorController:         doctorController,
	})

	PORT := os.Getenv("SERVER_PORT")

	r.Run(":" + PORT)

}
