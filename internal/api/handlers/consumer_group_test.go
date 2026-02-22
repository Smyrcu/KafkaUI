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

func TestConsumerGroupHandler_List_ClusterNotFound(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewConsumerGroupHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/nonexistent/consumer-groups", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestConsumerGroupHandler_Details_ClusterNotFound(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewConsumerGroupHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/nonexistent/consumer-groups/my-group", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	rctx.URLParams.Add("groupName", "my-group")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Details(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestConsumerGroupHandler_ResetOffsets_ClusterNotFound(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewConsumerGroupHandler(reg)

	payload := map[string]any{"topic": "my-topic", "resetTo": "earliest"}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/nonexistent/consumer-groups/my-group/reset", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	rctx.URLParams.Add("groupName", "my-group")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.ResetOffsets(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestConsumerGroupHandler_ResetOffsets_InvalidBody(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewConsumerGroupHandler(reg)

	body := bytes.NewBufferString("{invalid json}")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/consumer-groups/my-group/reset", body)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	rctx.URLParams.Add("groupName", "my-group")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.ResetOffsets(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestConsumerGroupHandler_ResetOffsets_MissingTopic(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewConsumerGroupHandler(reg)

	payload := map[string]any{"topic": "", "resetTo": "earliest"}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/consumer-groups/my-group/reset", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	rctx.URLParams.Add("groupName", "my-group")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.ResetOffsets(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestConsumerGroupHandler_ResetOffsets_InvalidResetTo(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewConsumerGroupHandler(reg)

	payload := map[string]any{"topic": "my-topic", "resetTo": "invalid-value"}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/consumer-groups/my-group/reset", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	rctx.URLParams.Add("groupName", "my-group")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.ResetOffsets(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}
