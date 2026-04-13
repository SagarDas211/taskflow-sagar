package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL   string
	JWTSecret     string
	ServerAddress string
	RedisAddress  string
	RedisPassword string
	RedisDB       int
}

func Load() (Config, error) {
	_ = godotenv.Load(".env", "../.env")

	cfg := Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		ServerAddress: envOrDefault("SERVER_ADDRESS", ":8080"),
		RedisAddress:  os.Getenv("REDIS_ADDRESS"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
	}

	redisDB, err := strconv.Atoi(envOrDefault("REDIS_DB", "0"))
	if err != nil {
		return Config{}, fmt.Errorf("REDIS_DB must be an integer")
	}
	cfg.RedisDB = redisDB

	switch {
	case cfg.DatabaseURL == "":
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	case cfg.JWTSecret == "":
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
