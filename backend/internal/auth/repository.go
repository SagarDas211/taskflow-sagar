package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrEmailAlreadyExists = errors.New("email already exists")

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user User) (User, error) {
	const query = `
		INSERT INTO users (id, name, email, password)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at
	`

	err := r.db.QueryRow(ctx, query, user.ID, user.Name, user.Email, user.Password).Scan(&user.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrEmailAlreadyExists
		}

		return User{}, fmt.Errorf("insert user: %w", err)
	}

	return user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (User, error) {
	const query = `
		SELECT id, name, email, password, created_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`

	var user User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Password,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrInvalidCredentials
		}

		return User{}, fmt.Errorf("get user by email: %w", err)
	}

	return user, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	return pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "email")
}

func withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 5*time.Second)
}
