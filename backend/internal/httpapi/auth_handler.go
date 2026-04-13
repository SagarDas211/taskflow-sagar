package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"taskflow/internal/auth"
)

type AuthHandler struct {
	authService *auth.Service
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func NewAuthHandler(authService *auth.Service) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var request registerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondValidationError(c, map[string]string{"body": "must be valid JSON"})
		return
	}

	if fields := validateRegisterRequest(request); len(fields) > 0 {
		respondValidationError(c, fields)
		return
	}

	user, err := h.authService.Register(c.Request.Context(), auth.RegisterInput{
		Name:     request.Name,
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrEmailAlreadyExists):
			respondValidationError(c, map[string]string{"email": "already exists"})
		default:
			respondError(c, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         user.ID,
		"name":       user.Name,
		"email":      user.Email,
		"created_at": user.CreatedAt,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondValidationError(c, map[string]string{"body": "must be valid JSON"})
		return
	}

	if fields := validateLoginRequest(request); len(fields) > 0 {
		respondValidationError(c, fields)
		return
	}

	token, err := h.authService.Login(c.Request.Context(), auth.LoginInput{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			respondError(c, http.StatusUnauthorized, "unauthenticated")
		default:
			respondError(c, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"access_token": token})
}

func validateRegisterRequest(request registerRequest) map[string]string {
	fields := map[string]string{}

	if strings.TrimSpace(request.Name) == "" {
		fields["name"] = "is required"
	}
	if strings.TrimSpace(request.Email) == "" {
		fields["email"] = "is required"
	} else if !strings.Contains(request.Email, "@") {
		fields["email"] = "must be a valid email"
	}
	if strings.TrimSpace(request.Password) == "" {
		fields["password"] = "is required"
	}

	return fields
}

func validateLoginRequest(request loginRequest) map[string]string {
	fields := map[string]string{}

	if strings.TrimSpace(request.Email) == "" {
		fields["email"] = "is required"
	}
	if strings.TrimSpace(request.Password) == "" {
		fields["password"] = "is required"
	}

	return fields
}
