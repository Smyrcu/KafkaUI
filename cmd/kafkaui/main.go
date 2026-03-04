package main

import (
	"context"
	"flag"
	"fmt"
	"io"
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

// spaHandler serves static files and falls back to index.html for client-side routes.
type spaHandler struct {
	fs       http.FileSystem
	fallback []byte
}

func newSPAHandler(fsys http.FileSystem) *spaHandler {
	index, err := fsys.Open("index.html")
	if err != nil {
		panic("frontend dist missing index.html: " + err.Error())
	}
	defer index.Close()
	data, err := io.ReadAll(index)
	if err != nil {
		panic("reading index.html: " + err.Error())
	}
	return &spaHandler{fs: fsys, fallback: data}
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	f, err := h.fs.Open(path)
	if err != nil {
		// File not found — serve index.html for SPA routing
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(h.fallback)
		return
	}
	f.Close()

	http.FileServer(h.fs).ServeHTTP(w, r)
}

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
		if cfg.Auth.Enabled {
			logger.Warn("auth is enabled but no session secret configured — using insecure default. Set auth.session.secret or SESSION_SECRET env var")
		}
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

	// Create basic authenticator if auth is enabled and type is basic
	var basicAuth *auth.BasicAuthenticator
	var rateLimiter *auth.LoginRateLimiter
	if cfg.Auth.Enabled && cfg.Auth.Type == "basic" {
		basicAuth = auth.NewBasicAuthenticator(cfg.Auth.Basic.Users)

		maxAttempts := cfg.Auth.Basic.RateLimit.MaxAttempts
		if maxAttempts == 0 {
			maxAttempts = 5
		}
		windowSecs := cfg.Auth.Basic.RateLimit.WindowSeconds
		if windowSecs == 0 {
			windowSecs = 60
		}
		rateLimiter = auth.NewLoginRateLimiter(maxAttempts, time.Duration(windowSecs)*time.Second)

		logger.Info("basic authentication enabled", "users", len(cfg.Auth.Basic.Users))
	}

	authType := ""
	if cfg.Auth.Enabled {
		authType = cfg.Auth.Type
	}

	router := api.NewRouter(registry, logger, sessions, cfg.Auth.Enabled, maskingEngine, authProvider, basicAuth, rateLimiter, authType)

	frontendContent, err := fs.Sub(fe.FS, "dist")
	if err != nil {
		logger.Error("failed to create frontend filesystem", "error", err)
		os.Exit(1)
	}
	spaHandler := newSPAHandler(http.FS(frontendContent))

	mux := http.NewServeMux()
	mux.Handle("/api/", router)
	mux.Handle("/ws/", router)
	mux.Handle("/", spaHandler)

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
