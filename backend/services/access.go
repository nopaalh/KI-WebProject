package services

import (
	"errors"
	"fmt"

	"project1/models"

	"gorm.io/gorm"
)

var ErrAccessForbidden = errors.New("forbidden")

type currentUserReader interface {
	FindByID(uint) (*models.User, error)
}

func authorizeUser(users currentUserReader, userID uint, roles ...string) (*models.User, error) {
	if userID == 0 {
		return nil, ErrInvalidUser
	}
	user, err := users.FindByID(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && user == nil) {
		return nil, ErrInvalidUser
	}
	if err != nil {
		return nil, fmt.Errorf("load current user: %w", err)
	}
	for _, role := range roles {
		if user.Role == role {
			return user, nil
		}
	}
	return nil, ErrAccessForbidden
}
