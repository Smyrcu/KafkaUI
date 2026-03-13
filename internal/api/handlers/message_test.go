package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/testutil"
)

func TestMessageHandler_Browse_ClusterNotFound(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewMessageHandler(reg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/nonexistent/topics/test/messages", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	rctx.URLParams.Add("topicName", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Browse(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestMessageHandler_Browse_InvalidPartition(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewMessageHandler(reg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/alpha/topics/test/messages?partition=abc", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	rctx.URLParams.Add("topicName", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Browse(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestMessageHandler_Browse_InvalidLimit(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewMessageHandler(reg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/alpha/topics/test/messages?limit=999", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	rctx.URLParams.Add("topicName", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Browse(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestMessageHandler_Produce_ClusterNotFound(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewMessageHandler(reg, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/nonexistent/topics/test/messages", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	rctx.URLParams.Add("topicName", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Produce(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestMessageHandler_Produce_InvalidBody(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewMessageHandler(reg, nil)

	body := bytes.NewBufferString("{invalid json}")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/topics/test/messages", body)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	rctx.URLParams.Add("topicName", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Produce(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestMessageHandler_Browse_InvalidOffset(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewMessageHandler(reg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/alpha/topics/test/messages?offset=notanumber", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	rctx.URLParams.Add("topicName", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Browse(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestMessageHandler_Browse_InvalidTimestamp(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewMessageHandler(reg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/alpha/topics/test/messages?timestamp=not-a-date", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	rctx.URLParams.Add("topicName", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Browse(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestMessageHandler_Browse_LimitTooLow(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewMessageHandler(reg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/alpha/topics/test/messages?limit=0", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	rctx.URLParams.Add("topicName", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Browse(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestMessageHandler_Browse_LimitNonNumeric(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewMessageHandler(reg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/alpha/topics/test/messages?limit=abc", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	rctx.URLParams.Add("topicName", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Browse(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestMessageHandler_Browse_ValidOffsetKeywords(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewMessageHandler(reg, nil)

	for _, offset := range []string{"earliest", "latest"} {
		t.Run(offset, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/alpha/topics/test/messages?offset="+offset, nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("clusterName", "alpha")
			rctx.URLParams.Add("topicName", "test")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rec := httptest.NewRecorder()
			h.Browse(rec, req)

			// Should be 500 (Kafka unreachable), not 400
			if rec.Code == http.StatusBadRequest {
				t.Fatalf("offset=%s should be valid, but got 400", offset)
			}
		})
	}
}
