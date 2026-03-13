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

func TestTopicHandler_List_ClusterNotFound(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewTopicHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/nonexistent/topics", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestTopicHandler_Create_InvalidBody(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewTopicHandler(reg)

	body := bytes.NewBufferString("{invalid json}")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/topics", body)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestTopicHandler_Create_MissingName(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewTopicHandler(reg)

	payload := map[string]any{"partitions": 3, "replicas": 1}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/topics", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestTopicHandler_List_ValidCluster(t *testing.T) {
	// With a valid cluster name, we expect either 200 (Kafka reachable) or 500 (unreachable), but never 404
	reg := testutil.MustCreateRegistry(t)
	h := NewTopicHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/alpha/topics", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code == http.StatusNotFound {
		t.Fatalf("valid cluster should not return 404, got %d", rec.Code)
	}
}

func TestTopicHandler_Details_ClusterNotFound(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewTopicHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/nonexistent/topics/test", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	rctx.URLParams.Add("topicName", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Details(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestTopicHandler_Delete_ClusterNotFound(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewTopicHandler(reg)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/clusters/nonexistent/topics/test", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	rctx.URLParams.Add("topicName", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestTopicHandler_Create_ClusterNotFound(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewTopicHandler(reg)

	payload := map[string]any{"name": "test", "partitions": 1, "replicas": 1}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/nonexistent/topics", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}
