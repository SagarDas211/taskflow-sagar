package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"taskflow/internal/auth"
	"taskflow/internal/cache"
	"taskflow/internal/config"
	"taskflow/internal/database"
	"taskflow/internal/httpapi"
	"taskflow/internal/project"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	dbPool, err := database.NewPostgresPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	userRepo := auth.NewUserRepository(dbPool)
	authService := auth.NewService(userRepo, cfg.JWTSecret, 24*time.Hour)
	projectService := project.NewService(project.NewRepository(dbPool), cache.NoopCache{})

	if cfg.RedisAddress != "" {
		redisCache, err := cache.NewRedisCache(cfg.RedisAddress, cfg.RedisPassword, cfg.RedisDB)
		if err != nil {
			logger.Warn("redis unavailable, continuing without cache", "error", err)
		} else {
			defer redisCache.Close()
			projectService = project.NewService(project.NewRepository(dbPool), redisCache)
			logger.Info("redis cache enabled", "addr", cfg.RedisAddress, "db", cfg.RedisDB)
		}
	}

	router := httpapi.NewRouter(logger, authService, projectService)

	server := &http.Server{
		Addr:              cfg.ServerAddress,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("starting server", "addr", cfg.ServerAddress)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped cleanly")
}
