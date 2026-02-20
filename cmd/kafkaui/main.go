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
	"github.com/Smyrcu/KafkaUI/internal/config"
	fe "github.com/Smyrcu/KafkaUI/internal/frontend"
	"github.com/Smyrcu/KafkaUI/internal/kafka"
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

	router := api.NewRouter(registry, logger)

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
