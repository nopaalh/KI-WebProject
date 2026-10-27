package services

import (
	"errors"

	"project1/models"
	"project1/utils"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

var ErrUsernameExist = errors.New("username already exist")
var InvalidCredentials = errors.New("Invalid password or username")

type userRepository interface {
	Create(*models.User) error
	FindByUsername(string) (*models.User, error)
	FindByID(uint) (*models.User, error)
}

type AuthService struct {
	UserRepo userRepository
}

func NewAuthService(userRepo userRepository) *AuthService {
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
		Role:         models.RolePatient,
	}

	err = s.UserRepo.Create(&user)
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return ErrUsernameExist
	}
	return err
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

func (s *AuthService) CurrentUser(userID uint) (*models.User, error) {
	return s.UserRepo.FindByID(userID)
}

func (s *AuthService) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	return utils.ValidateToken(tokenString)
}
