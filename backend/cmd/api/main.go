package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/weedpanel/weedpanel/backend/internal/config"
	"github.com/weedpanel/weedpanel/backend/internal/database"
	"github.com/weedpanel/weedpanel/backend/internal/httpapi"
	"github.com/weedpanel/weedpanel/backend/internal/security"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(0)
	}

	cfg := config.Load()
	if cfg.EncryptionKey != "" {
		if _, err := security.NewVault(cfg.EncryptionKey); err != nil {
			slog.Error("invalid encryption key", "error", err)
			os.Exit(1)
		}
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := database.Migrate(ctx, db); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	handler := httpapi.New(db, cfg)
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      0, // streaming endpoints will need long-lived responses
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("WeedPanel API listening", "port", cfg.Port, "env", cfg.Environment)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
