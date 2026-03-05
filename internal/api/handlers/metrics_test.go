package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/metrics"
)

func TestMetricsHandler_NotConfigured(t *testing.T) {
	h := NewMetricsHandler(nil, nil)

	r := chi.NewRouter()
	r.Get("/clusters/{clusterName}/metrics", h.Metrics)

	req := httptest.NewRequest(http.MethodGet, "/clusters/test/metrics", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestMetricsHandler_ClusterNotFound(t *testing.T) {
	scrapers := map[string]*metrics.Scraper{
		"nonexistent": metrics.NewScraper("http://localhost:1/metrics"),
	}
	h := NewMetricsHandler(nil, scrapers)

	r := chi.NewRouter()
	r.Get("/clusters/{clusterName}/metrics", h.Metrics)

	req := httptest.NewRequest(http.MethodGet, "/clusters/nonexistent/metrics", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}
