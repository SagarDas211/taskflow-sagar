package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

var ErrInvalidCredentials = errors.New("invalid credentials")

type Service struct {
	users       *UserRepository
	jwtSecret   []byte
	tokenExpiry time.Duration
}

type RegisterInput struct {
	Name     string
	Email    string
	Password string
}

type LoginInput struct {
	Email    string
	Password string
}

type TokenClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func NewService(users *UserRepository, jwtSecret string, tokenExpiry time.Duration) *Service {
	return &Service{
		users:       users,
		jwtSecret:   []byte(jwtSecret),
		tokenExpiry: tokenExpiry,
	}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (User, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcryptCost)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}

	user := User{
		ID:       uuid.NewString(),
		Name:     strings.TrimSpace(input.Name),
		Email:    strings.TrimSpace(strings.ToLower(input.Email)),
		Password: string(hashedPassword),
	}

	createdUser, err := s.users.Create(ctx, user)
	if err != nil {
		return User{}, err
	}

	createdUser.Password = ""
	return createdUser, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (string, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	user, err := s.users.GetByEmail(ctx, strings.TrimSpace(strings.ToLower(input.Email)))
	if err != nil {
		return "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		return "", ErrInvalidCredentials
	}

	now := time.Now()
	claims := TokenClaims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.tokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return signedToken, nil
}

func (s *Service) ParseToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}

		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidCredentials
	}

	return claims, nil
}
