package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Smyrcu/KafkaUI/internal/api"
	"github.com/Smyrcu/KafkaUI/internal/auth"
	"github.com/Smyrcu/KafkaUI/internal/config"
	fe "github.com/Smyrcu/KafkaUI/internal/frontend"
	"github.com/Smyrcu/KafkaUI/internal/kafka"
	"github.com/Smyrcu/KafkaUI/internal/masking"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	registry, err := kafka.NewRegistry(cfg)
	if err != nil {
		logger.Error("failed to create kafka registry", "error", err)
		os.Exit(1)
	}
	defer registry.Close()

	sessionSecret := cfg.Auth.Session.Secret
	if sessionSecret == "" {
		sessionSecret = "kafkaui-default-secret-change-me"
	}
	sessions := auth.NewSessionManager(sessionSecret, cfg.Auth.Session.MaxAge)

	// Create masking engine (nil-safe; handlers skip masking when nil rules)
	var maskingEngine *masking.Engine
	if len(cfg.DataMasking.Rules) > 0 {
		maskingEngine = masking.NewEngine(cfg.DataMasking)
		logger.Info("data masking enabled", "rules", len(cfg.DataMasking.Rules))
	}

	// Create OIDC provider if auth is enabled and type is oidc
	var authProvider *auth.Provider
	if cfg.Auth.Enabled && cfg.Auth.Type == "oidc" {
		var err error
		authProvider, err = auth.NewProvider(context.Background(), cfg.Auth.OIDC)
		if err != nil {
			logger.Error("failed to create OIDC provider", "error", err)
			os.Exit(1)
		}
		logger.Info("OIDC authentication enabled", "issuer", cfg.Auth.OIDC.Issuer)
	}

	router := api.NewRouter(registry, logger, sessions, cfg.Auth.Enabled, maskingEngine, authProvider)

	frontendContent, err := fs.Sub(fe.FS, "dist")
	if err != nil {
		logger.Error("failed to create frontend filesystem", "error", err)
		os.Exit(1)
	}
	fileServer := http.FileServer(http.FS(frontendContent))

	mux := http.NewServeMux()
	mux.Handle("/api/", router)
	mux.Handle("/ws/", router)
	mux.Handle("/", fileServer)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // disabled for WebSocket support
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("starting server", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}
	logger.Info("server stopped")
}
