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

func TestUserHandler_List_ClusterNotFound(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewUserHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/nonexistent/users", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestUserHandler_List_ValidCluster(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewUserHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/alpha/users", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.List(rec, req)

	// 500 = Kafka unreachable (not 404)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
}

func TestUserHandler_Create_ClusterNotFound(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewUserHandler(reg)

	payload := map[string]any{
		"name":      "alice",
		"password":  "secret123",
		"mechanism": "SCRAM-SHA-256",
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/nonexistent/users", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestUserHandler_Create_InvalidBody(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewUserHandler(reg)

	body := bytes.NewBufferString("{invalid json}")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/users", body)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestUserHandler_Create_MissingName(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewUserHandler(reg)

	payload := map[string]any{
		"password":  "secret123",
		"mechanism": "SCRAM-SHA-256",
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/users", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestUserHandler_Create_MissingPassword(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewUserHandler(reg)

	payload := map[string]any{
		"name":      "alice",
		"mechanism": "SCRAM-SHA-256",
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/users", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestUserHandler_Delete_ClusterNotFound(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewUserHandler(reg)

	payload := map[string]any{
		"name":      "alice",
		"mechanism": "SCRAM-SHA-256",
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/nonexistent/users/delete", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestUserHandler_Delete_InvalidBody(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewUserHandler(reg)

	body := bytes.NewBufferString("{invalid json}")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/users/delete", body)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestUserHandler_Delete_MissingName(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewUserHandler(reg)

	payload := map[string]any{
		"mechanism": "SCRAM-SHA-256",
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/users/delete", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestUserHandler_Delete_MissingMechanism(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewUserHandler(reg)

	payload := map[string]any{
		"name": "alice",
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/users/delete", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}
