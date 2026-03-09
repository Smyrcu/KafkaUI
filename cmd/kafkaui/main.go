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
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/Smyrcu/KafkaUI/internal/api"
	"github.com/Smyrcu/KafkaUI/internal/auth"
	"github.com/Smyrcu/KafkaUI/internal/config"
	fe "github.com/Smyrcu/KafkaUI/internal/frontend"
	"github.com/Smyrcu/KafkaUI/internal/kafka"
	"github.com/Smyrcu/KafkaUI/internal/masking"
	"github.com/Smyrcu/KafkaUI/internal/metrics"
)

// spaHandler serves static files and falls back to index.html for client-side routes.
type spaHandler struct {
	fs         http.FileSystem
	fallback   []byte
	fileServer http.Handler
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
	return &spaHandler{
		fs:         fsys,
		fallback:   data,
		fileServer: http.FileServer(fsys),
	}
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

	h.fileServer.ServeHTTP(w, r)
}

// mergeDynamicClusters loads clusters from the dynamic config file and
// appends any that don't collide with static cluster names.
func mergeDynamicClusters(cfg *config.Config, dynamicCfg *config.DynamicConfig, logger *slog.Logger) {
	dynClusters, err := dynamicCfg.Load()
	if err != nil {
		logger.Warn("failed to load dynamic config", "error", err)
		return
	}
	if len(dynClusters) == 0 {
		return
	}
	staticSet := make(map[string]bool, len(cfg.Clusters))
	for _, c := range cfg.Clusters {
		staticSet[c.Name] = true
	}
	for _, dc := range dynClusters {
		if !staticSet[dc.Name] {
			cfg.Clusters = append(cfg.Clusters, dc)
		}
	}
	logger.Info("loaded dynamic clusters", "count", len(dynClusters))
}

// initOIDCProviders creates OIDC providers when auth types include "oidc".
func initOIDCProviders(cfg *config.Config, logger *slog.Logger) (map[string]*auth.Provider, []config.OIDCProvider) {
	if !cfg.Auth.Enabled || !slices.Contains(cfg.Auth.Types, "oidc") {
		return nil, nil
	}
	providers := make(map[string]*auth.Provider)
	for _, p := range cfg.Auth.OIDC.Providers {
		provider, err := auth.NewProvider(context.Background(), p, cfg.Auth.OIDC.RedirectURL)
		if err != nil {
			logger.Error("failed to create OIDC provider", "name", p.Name, "error", err)
			os.Exit(1)
		}
		providers[p.Name] = provider
		logger.Info("OIDC provider enabled", "name", p.Name, "issuer", p.Issuer)
	}
	return providers, cfg.Auth.OIDC.Providers
}

// initBasicAuth creates the basic authenticator and rate limiter when auth types include "basic".
func initBasicAuth(cfg *config.Config, logger *slog.Logger) (*auth.BasicAuthenticator, *auth.LoginRateLimiter) {
	if !cfg.Auth.Enabled || !slices.Contains(cfg.Auth.Types, "basic") {
		return nil, nil
	}
	basicAuth := auth.NewBasicAuthenticator(cfg.Auth.Basic.Users)

	maxAttempts := cfg.Auth.Basic.RateLimit.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 5
	}
	windowSecs := cfg.Auth.Basic.RateLimit.WindowSeconds
	if windowSecs == 0 {
		windowSecs = 60
	}
	rateLimiter := auth.NewLoginRateLimiter(maxAttempts, time.Duration(windowSecs)*time.Second)

	logger.Info("basic authentication enabled", "users", len(cfg.Auth.Basic.Users))
	return basicAuth, rateLimiter
}

// initMetrics creates per-cluster metrics scrapers, a shared store, and
// starts the background collector if any scrapers are configured.
func initMetrics(cfg *config.Config, registry *kafka.Registry, logger *slog.Logger) (map[string]*metrics.Scraper, *metrics.Store) {
	scrapers := make(map[string]*metrics.Scraper)
	listers := make(map[string]metrics.BrokerLister)
	for _, cc := range cfg.Clusters {
		if cc.Metrics.URL != "" {
			scrapers[cc.Name] = metrics.NewScraper(cc.Metrics.URL)
			clusterName := cc.Name
			listers[clusterName] = func(ctx context.Context) ([]metrics.BrokerInfo, error) {
				client, ok := registry.Get(clusterName)
				if !ok {
					return nil, fmt.Errorf("cluster %q not found", clusterName)
				}
				brokers, err := client.Brokers(ctx)
				if err != nil {
					return nil, err
				}
				result := make([]metrics.BrokerInfo, len(brokers))
				for i, b := range brokers {
					result[i] = metrics.BrokerInfo{ID: b.ID, Host: b.Host}
				}
				return result, nil
			}
			logger.Info("metrics scraper enabled", "cluster", cc.Name, "url", cc.Metrics.URL)
		}
	}

	store := metrics.NewStore()
	if len(scrapers) > 0 {
		collector := metrics.NewCollector(store, scrapers, listers, logger)
		go collector.Run(context.Background())
		logger.Info("metrics collector started", "clusters", len(scrapers))
	}
	return scrapers, store
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	dynamicConfigPath := flag.String("dynamic-config", "", "path to dynamic config file (default: <config-dir>/dynamic.yaml)")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Collect static cluster names before merging dynamic
	staticClusterNames := make([]string, 0, len(cfg.Clusters))
	for _, c := range cfg.Clusters {
		staticClusterNames = append(staticClusterNames, c.Name)
	}

	// Load and merge dynamic clusters
	dynPath := *dynamicConfigPath
	if dynPath == "" {
		dynPath = filepath.Join(filepath.Dir(*configPath), "dynamic.yaml")
	}
	dynamicCfg := config.NewDynamicConfig(dynPath)
	mergeDynamicClusters(cfg, dynamicCfg, logger)

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

	oidcProviders, oidcProviderCfg := initOIDCProviders(cfg, logger)
	basicAuth, rateLimiter := initBasicAuth(cfg, logger)
	metricsScrapers, metricsStore := initMetrics(cfg, registry, logger)

	router := api.NewRouter(api.RouterDeps{
		Registry:           registry,
		Logger:             logger,
		Sessions:           sessions,
		AuthEnabled:        cfg.Auth.Enabled,
		MaskingEngine:      maskingEngine,
		OIDCProviders:      oidcProviders,
		OIDCProviderCfg:    oidcProviderCfg,
		BasicAuth:          basicAuth,
		RateLimiter:        rateLimiter,
		AuthTypes:          cfg.Auth.Types,
		MetricsScrapers:    metricsScrapers,
		MetricsStore:       metricsStore,
		DynamicCfg:         dynamicCfg,
		StaticClusterNames: staticClusterNames,
	})

	frontendContent, err := fs.Sub(fe.FS, "dist")
	if err != nil {
		logger.Error("failed to create frontend filesystem", "error", err)
		os.Exit(1)
	}
	spaHandler := newSPAHandler(http.FS(frontendContent))

	mux := http.NewServeMux()
	mux.Handle("/api/", router)
	mux.Handle("/ws/", router)
	mux.Handle("/healthz", router)
	mux.Handle("/readyz", router)
	mux.Handle("/readyz/", router)
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
