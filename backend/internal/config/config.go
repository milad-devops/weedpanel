package config

import (
	"log/slog"
	"os"
)

type Config struct {
	Environment   string
	Port          string
	DatabaseURL   string
	RedisURL      string
	EncryptionKey string
	SessionSecret string
	LogLevel      slog.Level
}

func Load() Config {
	level := slog.LevelInfo
	switch os.Getenv("APP_LOG_LEVEL") {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return Config{
		Environment:   env("APP_ENV", "development"),
		Port:          env("APP_PORT", "8081"),
		DatabaseURL:   env("DATABASE_URL", "postgres://weedpanel:weedpanel@localhost:5432/weedpanel?sslmode=disable"),
		RedisURL:      env("REDIS_URL", "redis://localhost:6379/0"),
		EncryptionKey: os.Getenv("APP_ENCRYPTION_KEY"),
		SessionSecret: os.Getenv("SESSION_SECRET"),
		LogLevel:      level,
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
