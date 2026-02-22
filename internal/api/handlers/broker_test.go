package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestBrokerHandler_List_ClusterNotFound(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewBrokerHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/nonexistent/brokers", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestBrokerHandler_List_ValidCluster(t *testing.T) {
	// With a valid cluster name (no real Kafka), we expect 500 not 404
	reg := mustCreateRegistry(t)
	h := NewBrokerHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/alpha/brokers", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.List(rec, req)

	// Should return 500 (Kafka unreachable), not 404
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
}
