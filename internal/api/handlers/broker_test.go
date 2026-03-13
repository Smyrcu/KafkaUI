package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/testutil"
)

func TestBrokerHandler_List_ClusterNotFound(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
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
	// With a valid cluster name, we expect either 200 (Kafka reachable) or 500 (unreachable), but never 404
	reg := testutil.MustCreateRegistry(t)
	h := NewBrokerHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/alpha/brokers", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code == http.StatusNotFound {
		t.Fatalf("valid cluster should not return 404, got %d", rec.Code)
	}
}
