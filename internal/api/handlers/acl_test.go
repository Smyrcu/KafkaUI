package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/testutil"
)

func TestACLHandler_List_ClusterNotFound(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewACLHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/nonexistent/acls", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestACLHandler_List_ValidCluster(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewACLHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/alpha/acls", nil)
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

func TestACLHandler_Create_ClusterNotFound(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewACLHandler(reg)

	payload := map[string]any{
		"resourceType": "TOPIC",
		"resourceName": "test-topic",
		"patternType":  "LITERAL",
		"principal":    "User:alice",
		"host":         "*",
		"operation":    "READ",
		"permission":   "ALLOW",
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/nonexistent/acls", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestACLHandler_Create_InvalidBody(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewACLHandler(reg)

	body := bytes.NewBufferString("{invalid json}")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/acls", body)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestACLHandler_Create_MissingResourceType(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewACLHandler(reg)

	payload := map[string]any{
		"resourceName": "test-topic",
		"patternType":  "LITERAL",
		"principal":    "User:alice",
		"host":         "*",
		"operation":    "READ",
		"permission":   "ALLOW",
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/acls", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestACLHandler_Create_MissingPrincipal(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewACLHandler(reg)

	payload := map[string]any{
		"resourceType": "TOPIC",
		"resourceName": "test-topic",
		"patternType":  "LITERAL",
		"host":         "*",
		"operation":    "READ",
		"permission":   "ALLOW",
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/acls", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestACLHandler_Create_MissingOperation(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewACLHandler(reg)

	payload := map[string]any{
		"resourceType": "TOPIC",
		"resourceName": "test-topic",
		"patternType":  "LITERAL",
		"principal":    "User:alice",
		"host":         "*",
		"permission":   "ALLOW",
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/acls", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestACLHandler_Delete_ClusterNotFound(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewACLHandler(reg)

	payload := map[string]any{
		"resourceType": "TOPIC",
		"resourceName": "test-topic",
		"patternType":  "LITERAL",
		"principal":    "User:alice",
		"host":         "*",
		"operation":    "READ",
		"permission":   "ALLOW",
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/nonexistent/acls/delete", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestACLHandler_Delete_InvalidBody(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewACLHandler(reg)

	body := bytes.NewBufferString("{invalid json}")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/acls/delete", body)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestACLHandler_Delete_MissingResourceType(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewACLHandler(reg)

	payload := map[string]any{
		"resourceName": "test-topic",
		"patternType":  "LITERAL",
		"principal":    "User:alice",
		"host":         "*",
		"operation":    "READ",
		"permission":   "ALLOW",
	}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/acls/delete", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}
