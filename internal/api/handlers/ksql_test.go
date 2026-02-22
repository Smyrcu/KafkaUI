package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestKsqlHandler_Execute_ClusterNotFound(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewKsqlHandler(reg)

	payload := map[string]any{"query": "SHOW STREAMS;"}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/nonexistent/ksql", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Execute(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestKsqlHandler_Execute_NoKsqlConfigured(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewKsqlHandler(reg)

	payload := map[string]any{"query": "SHOW STREAMS;"}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/ksql", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Execute(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestKsqlHandler_Execute_InvalidBody(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewKsqlHandler(reg)

	body := bytes.NewBufferString("{invalid json}")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/ksql", body)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Execute(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestKsqlHandler_Execute_EmptyQuery(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewKsqlHandler(reg)

	payload := map[string]any{"query": ""}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/ksql", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Execute(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestKsqlHandler_Info_ClusterNotFound(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewKsqlHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/nonexistent/ksql/info", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Info(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestKsqlHandler_Info_NoKsqlConfigured(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewKsqlHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/alpha/ksql/info", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Info(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}
