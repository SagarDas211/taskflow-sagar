package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"taskflow/internal/auth"
	"taskflow/internal/cache"
	"taskflow/internal/database"
	"taskflow/internal/project"
)

func TestAuthFlow(t *testing.T) {
	_ = godotenv.Load(".env", "../.env", "../../.env", "../../../.env")

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for integration test")
	}

	ctx := context.Background()
	dbPool, err := database.NewPostgresPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	defer dbPool.Close()

	resetUsersTable(t, dbPool)

	authService := auth.NewService(auth.NewUserRepository(dbPool), "test-secret-for-auth-flow", 24*time.Hour)
	projectService := project.NewService(project.NewRepository(dbPool), cache.NoopCache{})
	router := NewRouter(slog.New(slog.NewJSONHandler(os.Stdout, nil)), authService, projectService)

	registerBody := `{"name":"Sagar","email":"sagar@example.com","password":"Sagar@1234"}`
	registerRequest := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(registerBody))
	registerRequest.Header.Set("Content-Type", "application/json")
	registerRecorder := httptest.NewRecorder()

	router.ServeHTTP(registerRecorder, registerRequest)

	if registerRecorder.Code != http.StatusCreated {
		t.Fatalf("expected register status %d, got %d: %s", http.StatusCreated, registerRecorder.Code, registerRecorder.Body.String())
	}

	loginBody := `{"email":"sagar@example.com","password":"Sagar@1234"}`
	loginRequest := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(loginBody))
	loginRequest.Header.Set("Content-Type", "application/json")
	loginRecorder := httptest.NewRecorder()

	router.ServeHTTP(loginRecorder, loginRequest)

	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("expected login status %d, got %d: %s", http.StatusOK, loginRecorder.Code, loginRecorder.Body.String())
	}

	var loginResponse struct {
		AccessToken string `json:"access_token"`
	}

	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &loginResponse); err != nil {
		t.Fatalf("decode login response: %v", err)
	}

	if loginResponse.AccessToken == "" {
		t.Fatal("expected access token in login response")
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	meRequest.Header.Set("Authorization", "Bearer "+loginResponse.AccessToken)
	meRecorder := httptest.NewRecorder()

	router.ServeHTTP(meRecorder, meRequest)

	if meRecorder.Code != http.StatusOK {
		t.Fatalf("expected me status %d, got %d: %s", http.StatusOK, meRecorder.Code, meRecorder.Body.String())
	}

	var passwordHash string
	err = dbPool.QueryRow(ctx, `SELECT password FROM users WHERE LOWER(email) = LOWER($1)`, "sagar@example.com").Scan(&passwordHash)
	if err != nil {
		t.Fatalf("query hashed password: %v", err)
	}

	if passwordHash == "Sagar@1234" {
		t.Fatal("password was stored in plaintext")
	}
}

func resetUsersTable(t *testing.T, dbPool *pgxpool.Pool) {
	t.Helper()

	if _, err := dbPool.Exec(context.Background(), `TRUNCATE TABLE users CASCADE`); err != nil {
		t.Fatalf("truncate users table: %v", err)
	}
}
