package service

import (
	"project1/models"
	"project1/repositories"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	UserRepo *repositories.UserRepository
}

func NewAuthService(userRepo *repositories.UserRepository) *AuthService {
	return &AuthService{
		UserRepo: userRepo,
	}
}

func (s *AuthService) Register(name string, username string, password string) error {
	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)

	if err != nil {
		return err
	}

	user := models.User{
		Name:         name,
		Username:     username,
		PasswordHash: string(passwordHash),
	}

	return s.UserRepo.Create(&user)
}
