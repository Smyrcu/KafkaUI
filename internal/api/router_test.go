package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Smyrcu/KafkaUI/internal/auth"
	"github.com/Smyrcu/KafkaUI/internal/config"
	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	cfg := &config.Config{
		Clusters: []config.ClusterConfig{
			{Name: "test-cluster", BootstrapServers: "localhost:9092"},
		},
	}
	reg, err := kafka.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}
	t.Cleanup(reg.Close)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sessions := auth.NewSessionManager("test-secret", 3600)
	return NewRouter(reg, logger, sessions, false, nil, nil, nil, nil, nil)
}

func TestRouter_ClusterRoutes(t *testing.T) {
	router := newTestRouter(t)

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{
			name:       "GET /api/v1/clusters returns 200",
			method:     http.MethodGet,
			path:       "/api/v1/clusters",
			wantStatus: http.StatusOK,
		},
		{
			name:       "GET /api/v1/clusters/nonexistent/brokers returns 404",
			method:     http.MethodGet,
			path:       "/api/v1/clusters/nonexistent/brokers",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "GET /api/v1/clusters/nonexistent/topics returns 404",
			method:     http.MethodGet,
			path:       "/api/v1/clusters/nonexistent/topics",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "POST /api/v1/clusters/nonexistent/topics returns 404",
			method:     http.MethodPost,
			path:       "/api/v1/clusters/nonexistent/topics",
			body:       "{}",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "GET /api/v1/clusters/nonexistent/topics/test returns 404",
			method:     http.MethodGet,
			path:       "/api/v1/clusters/nonexistent/topics/test",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "DELETE /api/v1/clusters/nonexistent/topics/test returns 404",
			method:     http.MethodDelete,
			path:       "/api/v1/clusters/nonexistent/topics/test",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "GET /api/v1/clusters/nonexistent/topics/test/messages returns 404",
			method:     http.MethodGet,
			path:       "/api/v1/clusters/nonexistent/topics/test/messages",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "POST /api/v1/clusters/nonexistent/topics/test/messages returns 404",
			method:     http.MethodPost,
			path:       "/api/v1/clusters/nonexistent/topics/test/messages",
			body:       "{}",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "GET /api/v1/clusters/nonexistent/consumer-groups returns 404",
			method:     http.MethodGet,
			path:       "/api/v1/clusters/nonexistent/consumer-groups",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "GET /api/v1/clusters/nonexistent/consumer-groups/test returns 404",
			method:     http.MethodGet,
			path:       "/api/v1/clusters/nonexistent/consumer-groups/test",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "POST /api/v1/clusters/nonexistent/consumer-groups/test/reset returns 404",
			method:     http.MethodPost,
			path:       "/api/v1/clusters/nonexistent/consumer-groups/test/reset",
			body:       "{}",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d (body: %s)", tt.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRouter_DocsRoutes(t *testing.T) {
	router := newTestRouter(t)

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "GET /api/v1/docs returns 200",
			path:       "/api/v1/docs",
			wantStatus: http.StatusOK,
		},
		{
			name:       "GET /api/v1/docs/openapi.yaml returns 200",
			path:       "/api/v1/docs/openapi.yaml",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d (body: %s)", tt.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRouter_NotFoundRoutes(t *testing.T) {
	router := newTestRouter(t)

	tests := []struct {
		name           string
		method         string
		path           string
		wantStatusAny  []int
	}{
		{
			name:          "GET /api/v1/nonexistent returns 404 or 405",
			method:        http.MethodGet,
			path:          "/api/v1/nonexistent",
			wantStatusAny: []int{http.StatusNotFound, http.StatusMethodNotAllowed},
		},
		{
			name:          "PUT /api/v1/clusters returns 405",
			method:        http.MethodPut,
			path:          "/api/v1/clusters",
			wantStatusAny: []int{http.StatusMethodNotAllowed},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			found := false
			for _, want := range tt.wantStatusAny {
				if rec.Code == want {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected status to be one of %v, got %d (body: %s)", tt.wantStatusAny, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRouter_CORS(t *testing.T) {
	router := newTestRouter(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/clusters", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin to be \"*\", got %q", origin)
	}
}
