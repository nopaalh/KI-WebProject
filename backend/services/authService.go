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
	FindByUsernameIndex([]byte) (*models.User, error)
	FindByID(uint) (*models.User, error)
}

type AuthService struct {
	UserRepo userRepository
	key      [32]byte
}

func NewAuthService(userRepo userRepository, key [32]byte) *AuthService {
	return &AuthService{
		UserRepo: userRepo,
		key:      key,
	}
}

// UserView adalah data akun yang sudah didekripsi untuk menjawab /api/users/me
type UserView struct {
	ID       uint
	Name     string
	Username string
	Role     string
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
		UsernameIndex: usernameIndex(s.key, username),
		PasswordHash:  string(passwordHash),
		Role:          models.RolePatient,
	}
	user.NameAES, user.NameDES, user.NameRC4, err = encryptTriple(s.key, []byte(name))
	if err != nil {
		return err
	}
	user.UsernameAES, user.UsernameDES, user.UsernameRC4, err = encryptTriple(s.key, []byte(username))
	if err != nil {
		return err
	}

	err = s.UserRepo.Create(&user)
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return ErrUsernameExist
	}
	return err
}

func (s *AuthService) Login(username string, password string) (string, error) {
	user, err := s.UserRepo.FindByUsernameIndex(usernameIndex(s.key, username))

	if err != nil {
		return "", InvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash), []byte(password),
	)

	if err != nil {
		return "", InvalidCredentials
	}

	token, err := utils.GenerateToken(user.ID, username)
	if err != nil {
		return "", InvalidCredentials
	}

	return token, nil
}

func (s *AuthService) CurrentUser(userID uint) (*UserView, error) {
	user, err := s.UserRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	name, err := decryptTriple(s.key, user.NameAES, user.NameDES, user.NameRC4)
	if err != nil {
		return nil, err
	}
	username, err := decryptTriple(s.key, user.UsernameAES, user.UsernameDES, user.UsernameRC4)
	if err != nil {
		return nil, err
	}
	return &UserView{ID: user.ID, Name: string(name), Username: string(username), Role: user.Role}, nil
}

func (s *AuthService) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	return utils.ValidateToken(tokenString)
}
