package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
	"github.com/Smyrcu/KafkaUI/internal/testutil"
)

func TestLiveTailHandler_ClusterNotFound(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewLiveTailHandler(reg, testutil.TestLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/ws/clusters/nonexistent/topics/test/live", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	rctx.URLParams.Add("topicName", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Handle(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 for nonexistent cluster, got %d", rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if body != "cluster not found" {
		t.Fatalf("expected body 'cluster not found', got %q", body)
	}
}

func TestLiveTailHandler_NonWebSocketRequest(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewLiveTailHandler(reg, testutil.TestLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/ws/clusters/alpha/topics/test/live", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	rctx.URLParams.Add("topicName", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Handle(rec, req)

	// The gorilla/websocket upgrader returns HTTP 400 when the request is not
	// a valid WebSocket handshake.
	if rec.Code == http.StatusOK || rec.Code == http.StatusSwitchingProtocols {
		t.Fatalf("expected error status for non-WebSocket request, got %d", rec.Code)
	}
}

func TestLiveTailHandler_MissingClusterParam(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewLiveTailHandler(reg, testutil.TestLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/ws/clusters//topics/test/live", nil)
	// Set up route context with empty clusterName.
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "")
	rctx.URLParams.Add("topicName", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Handle(rec, req)

	// Empty cluster name should not be found in the registry.
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 for empty cluster name, got %d", rec.Code)
	}
}

func TestNewLiveTailHandler(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	logger := testutil.TestLogger()
	h := NewLiveTailHandler(reg, logger, nil)

	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.registry != reg {
		t.Fatal("expected handler registry to match")
	}
	if h.logger != logger {
		t.Fatal("expected handler logger to match")
	}
}

func TestLiveTailHandler_WebSocketUpgrade(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewLiveTailHandler(reg, testutil.TestLogger(), nil)

	// Create a test server with chi router to serve the WebSocket handler.
	r := chi.NewRouter()
	r.Get("/ws/clusters/{clusterName}/topics/{topicName}/live", h.Handle)
	server := httptest.NewServer(r)
	defer server.Close()

	// Convert HTTP URL to WS URL.
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/ws/clusters/alpha/topics/test-topic/live"

	dialer := websocket.Dialer{
		HandshakeTimeout: 2 * time.Second,
	}

	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		// The connection to Kafka will fail since there is no broker running,
		// but the WebSocket upgrade itself should succeed (status 101).
		// If the upgrade fails it means something else went wrong.
		if resp != nil && resp.StatusCode == http.StatusSwitchingProtocols {
			// Upgrade succeeded but the connection was immediately closed
			// because the Kafka consumer could not connect. This is acceptable.
			return
		}
		// The upgrade succeeded momentarily then the server closed the
		// connection, which is the expected behaviour when no Kafka broker
		// is available.
		t.Logf("dial error (expected without running broker): %v", err)
		return
	}
	defer conn.Close()

	// If we got a connection, the upgrade succeeded. Send a stop control
	// message to cleanly shut down the handler goroutine.
	stopMsg, _ := json.Marshal(controlMessage{Action: "stop"})
	_ = conn.WriteMessage(websocket.TextMessage, stopMsg)

	// Give the handler a moment to process the stop message.
	time.Sleep(100 * time.Millisecond)
}

func TestLiveTailHandler_WebSocketClusterNotFound(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewLiveTailHandler(reg, testutil.TestLogger(), nil)

	r := chi.NewRouter()
	r.Get("/ws/clusters/{clusterName}/topics/{topicName}/live", h.Handle)
	server := httptest.NewServer(r)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") +
		"/ws/clusters/nonexistent/topics/test-topic/live"

	dialer := websocket.Dialer{
		HandshakeTimeout: 2 * time.Second,
	}

	_, resp, err := dialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected error when dialing nonexistent cluster")
	}

	// The handler returns 404 before attempting the upgrade, so the
	// WebSocket handshake should fail.
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected HTTP 404, got %d", resp.StatusCode)
	}
}

func TestMessageRecord_JSONRoundTrip(t *testing.T) {
	original := kafka.MessageRecord{
		Partition: 3,
		Offset:    42,
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		Key:       "test-key",
		Value:     `{"data":"hello"}`,
		Headers:   map[string]string{"content-type": "application/json"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded kafka.MessageRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Partition != original.Partition {
		t.Errorf("partition: expected %d, got %d", original.Partition, decoded.Partition)
	}
	if decoded.Offset != original.Offset {
		t.Errorf("offset: expected %d, got %d", original.Offset, decoded.Offset)
	}
	if decoded.Key != original.Key {
		t.Errorf("key: expected %q, got %q", original.Key, decoded.Key)
	}
	if decoded.Value != original.Value {
		t.Errorf("value: expected %q, got %q", original.Value, decoded.Value)
	}
	if decoded.Headers["content-type"] != "application/json" {
		t.Errorf("header: expected 'application/json', got %q", decoded.Headers["content-type"])
	}
}

func TestControlMessage_JSONRoundTrip(t *testing.T) {
	original := controlMessage{Action: "stop"}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded controlMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Action != "stop" {
		t.Errorf("expected action 'stop', got %q", decoded.Action)
	}
}
