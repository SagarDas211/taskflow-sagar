package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"taskflow/internal/auth"
	"taskflow/internal/project"
)

func NewRouter(logger *slog.Logger, authService *auth.Service, projectService *project.Service) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(LoggingMiddleware(logger))

	authHandler := NewAuthHandler(authService)
	projectHandler := NewProjectHandler(projectService)

	router.NoRoute(func(c *gin.Context) {
		respondError(c, http.StatusNotFound, "not found")
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.POST("/auth/register", authHandler.Register)
	router.POST("/auth/login", authHandler.Login)

	protected := router.Group("/")
	protected.Use(AuthMiddleware(authService))
	protected.GET("/auth/me", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id": c.GetString("user_id"),
			"email":   c.GetString("email"),
		})
	})
	protected.GET("/projects", projectHandler.ListProjects)
	protected.POST("/projects", projectHandler.CreateProject)
	protected.GET("/projects/:id", projectHandler.GetProject)
	protected.PATCH("/projects/:id", projectHandler.UpdateProject)
	protected.DELETE("/projects/:id", projectHandler.DeleteProject)
	protected.GET("/projects/:id/tasks", projectHandler.ListTasks)
	protected.POST("/projects/:id/tasks", projectHandler.CreateTask)
	protected.GET("/projects/:id/stats", projectHandler.Stats)
	protected.PATCH("/tasks/:id", projectHandler.UpdateTask)
	protected.DELETE("/tasks/:id", projectHandler.DeleteTask)

	return router
}
