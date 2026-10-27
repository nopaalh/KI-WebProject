package controllers

import (
	"errors"
	"net/http"

	"project1/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuthController struct {
	AuthService *services.AuthService
}

func NewAuthController(authService *services.AuthService) *AuthController {
	return &AuthController{
		AuthService: authService,
	}
}

type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (c *AuthController) Register(ctx *gin.Context) {
	var req RegisterRequest

	err := ctx.ShouldBindJSON(&req)

	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
		})
		return
	}

	err = c.AuthService.Register(
		req.Name,
		req.Username,
		req.Password,
	)

	if err != nil {
		if errors.Is(err, services.ErrUsernameExist) {
			ctx.JSON(http.StatusConflict, gin.H{
				"message": err.Error(),
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
	})
}

func (c *AuthController) Me(ctx *gin.Context) {
	value, _ := ctx.Get("userID")
	userID, ok := value.(uint)
	if !ok || userID == 0 {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}

	user, err := c.AuthService.CurrentUser(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ctx.JSON(http.StatusUnauthorized, gin.H{"message": "User no longer exists"})
		return
	}
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to load current user"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"id":       user.ID,
		"name":     user.Name,
		"username": user.Username,
		"role":     user.Role,
	})
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (c *AuthController) Login(ctx *gin.Context) {
	var req LoginRequest

	err := ctx.ShouldBindJSON(&req)

	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid request",
		})
		return
	}

	token, err := c.AuthService.Login(
		req.Username,
		req.Password,
	)

	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"message": err.Error(),
		})
		return
	}

	ctx.SetSameSite(http.SameSiteLaxMode)

	ctx.SetCookie(
		"access_token",
		token,
		3600*24,
		"/",
		"",
		false,
		true,
	)

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
	})
}
