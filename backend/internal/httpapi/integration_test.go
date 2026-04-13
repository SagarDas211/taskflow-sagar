package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"taskflow/internal/auth"
	"taskflow/internal/cache"
	"taskflow/internal/database"
	"taskflow/internal/project"
)

func TestProjectLifecycle(t *testing.T) {
	router, cleanup := setupIntegrationRouter(t)
	defer cleanup()

	token := registerAndLogin(t, router, "owner@example.com")

	projectResp := performJSON(t, router, http.MethodPost, "/projects", token, map[string]any{
		"name":        "Project A",
		"description": "Demo project",
	})
	if projectResp.Code != http.StatusCreated {
		t.Fatalf("expected create project status %d, got %d: %s", http.StatusCreated, projectResp.Code, projectResp.Body.String())
	}

	var projectBody map[string]any
	decodeJSONBody(t, projectResp, &projectBody)
	projectID := projectBody["id"].(string)

	taskResp := performJSON(t, router, http.MethodPost, "/projects/"+projectID+"/tasks", token, map[string]any{
		"title":    "Task 1",
		"status":   "todo",
		"priority": "high",
	})
	if taskResp.Code != http.StatusCreated {
		t.Fatalf("expected create task status %d, got %d: %s", http.StatusCreated, taskResp.Code, taskResp.Body.String())
	}

	getResp := performRequest(t, router, http.MethodGet, "/projects/"+projectID, token, nil)
	if getResp.Code != http.StatusOK {
		t.Fatalf("expected project details status %d, got %d: %s", http.StatusOK, getResp.Code, getResp.Body.String())
	}

	statsResp := performRequest(t, router, http.MethodGet, "/projects/"+projectID+"/stats", token, nil)
	if statsResp.Code != http.StatusOK {
		t.Fatalf("expected stats status %d, got %d: %s", http.StatusOK, statsResp.Code, statsResp.Body.String())
	}
}

func TestProjectOwnerAuthorization(t *testing.T) {
	router, cleanup := setupIntegrationRouter(t)
	defer cleanup()

	ownerToken := registerAndLogin(t, router, "project-owner@example.com")
	otherToken := registerAndLogin(t, router, "other-user@example.com")

	projectResp := performJSON(t, router, http.MethodPost, "/projects", ownerToken, map[string]any{"name": "Owner Project"})
	if projectResp.Code != http.StatusCreated {
		t.Fatalf("expected create project status %d, got %d: %s", http.StatusCreated, projectResp.Code, projectResp.Body.String())
	}

	var projectBody map[string]any
	decodeJSONBody(t, projectResp, &projectBody)
	projectID := projectBody["id"].(string)

	updateResp := performJSON(t, router, http.MethodPatch, "/projects/"+projectID, otherToken, map[string]any{"name": "Changed"})
	if updateResp.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden status %d, got %d: %s", http.StatusForbidden, updateResp.Code, updateResp.Body.String())
	}
}

func TestTaskFiltersAndDeleteAuthorization(t *testing.T) {
	router, cleanup := setupIntegrationRouter(t)
	defer cleanup()

	ownerToken := registerAndLogin(t, router, "owner2@example.com")
	assigneeToken := registerAndLogin(t, router, "assignee@example.com")
	intruderToken := registerAndLogin(t, router, "intruder@example.com")

	projectResp := performJSON(t, router, http.MethodPost, "/projects", ownerToken, map[string]any{"name": "Filter Project"})
	if projectResp.Code != http.StatusCreated {
		t.Fatalf("expected create project status %d, got %d: %s", http.StatusCreated, projectResp.Code, projectResp.Body.String())
	}

	var projectBody map[string]any
	decodeJSONBody(t, projectResp, &projectBody)
	projectID := projectBody["id"].(string)

	assigneeID := lookupUserID(t, router, assigneeToken)

	taskResp := performJSON(t, router, http.MethodPost, "/projects/"+projectID+"/tasks", ownerToken, map[string]any{
		"title":       "Assigned Task",
		"status":      "in_progress",
		"priority":    "medium",
		"assignee_id": assigneeID,
		"due_date":    "2026-05-01",
	})
	if taskResp.Code != http.StatusCreated {
		t.Fatalf("expected create task status %d, got %d: %s", http.StatusCreated, taskResp.Code, taskResp.Body.String())
	}

	var taskBody map[string]any
	decodeJSONBody(t, taskResp, &taskBody)
	taskID := taskBody["id"].(string)

	listResp := performRequest(t, router, http.MethodGet, "/projects/"+projectID+"/tasks?status=in_progress&assignee="+assigneeID+"&page=1&limit=5", assigneeToken, nil)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected task list status %d, got %d: %s", http.StatusOK, listResp.Code, listResp.Body.String())
	}

	deleteResp := performRequest(t, router, http.MethodDelete, "/tasks/"+taskID, intruderToken, nil)
	if deleteResp.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden delete status %d, got %d: %s", http.StatusForbidden, deleteResp.Code, deleteResp.Body.String())
	}

	deleteOwnerResp := performRequest(t, router, http.MethodDelete, "/tasks/"+taskID, ownerToken, nil)
	if deleteOwnerResp.Code != http.StatusOK {
		t.Fatalf("expected owner delete status %d, got %d: %s", http.StatusOK, deleteOwnerResp.Code, deleteOwnerResp.Body.String())
	}
}

func setupIntegrationRouter(t *testing.T) (*gin.Engine, func()) {
	t.Helper()

	_ = godotenv.Load(".env", "../.env", "../../.env", "../../../.env")

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for integration tests")
	}

	dbPool, err := database.NewPostgresPool(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}

	resetDatabase(t, dbPool)

	authService := auth.NewService(auth.NewUserRepository(dbPool), "test-secret-for-auth-flow", 24*time.Hour)
	projectService := project.NewService(project.NewRepository(dbPool), cache.NoopCache{})
	router := NewRouter(slog.New(slog.NewJSONHandler(os.Stdout, nil)), authService, projectService)

	return router, func() { dbPool.Close() }
}

func resetDatabase(t *testing.T, dbPool *pgxpool.Pool) {
	t.Helper()

	if _, err := dbPool.Exec(context.Background(), `TRUNCATE TABLE tasks, projects, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("reset database: %v", err)
	}
}

func registerAndLogin(t *testing.T, router http.Handler, email string) string {
	t.Helper()

	registerResp := performJSON(t, router, http.MethodPost, "/auth/register", "", map[string]any{
		"name":     "Test User",
		"email":    email,
		"password": "Sagar@1234",
	})
	if registerResp.Code != http.StatusCreated {
		t.Fatalf("expected register status %d, got %d: %s", http.StatusCreated, registerResp.Code, registerResp.Body.String())
	}

	loginResp := performJSON(t, router, http.MethodPost, "/auth/login", "", map[string]any{
		"email":    email,
		"password": "Sagar@1234",
	})
	if loginResp.Code != http.StatusOK {
		t.Fatalf("expected login status %d, got %d: %s", http.StatusOK, loginResp.Code, loginResp.Body.String())
	}

	var payload struct {
		AccessToken string `json:"access_token"`
	}
	decodeJSONBody(t, loginResp, &payload)
	return payload.AccessToken
}

func lookupUserID(t *testing.T, router http.Handler, token string) string {
	t.Helper()

	resp := performRequest(t, router, http.MethodGet, "/auth/me", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected me status %d, got %d: %s", http.StatusOK, resp.Code, resp.Body.String())
	}

	var payload map[string]string
	decodeJSONBody(t, resp, &payload)
	return payload["user_id"]
}

func performJSON(t *testing.T, router http.Handler, method, path, token string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	return performRequest(t, router, method, path, token, bytes.NewReader(payload))
}

func performRequest(t *testing.T, router http.Handler, method, path, token string, body *bytes.Reader) *httptest.ResponseRecorder {
	t.Helper()

	if body == nil {
		body = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func decodeJSONBody(t *testing.T, recorder *httptest.ResponseRecorder, dest any) {
	t.Helper()

	if err := json.Unmarshal(recorder.Body.Bytes(), dest); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}
