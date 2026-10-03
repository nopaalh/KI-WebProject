package services

import (
	"errors"
	"project1/models"
	"project1/repositories"
	"project1/utils"

	"golang.org/x/crypto/bcrypt"
)

var ErrUsernameExist = errors.New("username already exist")
var InvalidCredentials = errors.New("Invalid password or username")

type AuthService struct {
	UserRepo *repositories.UserRepository
}

func NewAuthService(userRepo *repositories.UserRepository) *AuthService {
	return &AuthService{
		UserRepo: userRepo,
	}
}

func (s *AuthService) Register(name string, username string, password string) error {
	// 	existingUser, err := s.UserRepo.FindByUsername(username)
	//
	// 	if err == nil && existingUser != nil {
	// 		return fmt.Errorf("%w", ErrUsernameExist)
	// 	}

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

func (s *AuthService) Login(username string, password string) (string, error) {
	user, err := s.UserRepo.FindByUsername(username)

	if err != nil {
		return "", InvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash), []byte(password),
	)

	if err != nil {
		return "", InvalidCredentials
	}

	token, err := utils.GenerateToken(user.ID, user.Username)
	if err != nil {
		return "", InvalidCredentials
	}

	return token, nil
}
