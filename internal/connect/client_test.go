package connect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_ListConnectors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/connectors" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"my-source": map[string]any{
				"info":   map[string]any{"name": "my-source", "config": map[string]any{}, "type": "source"},
				"status": map[string]any{"name": "my-source", "connector": map[string]any{"state": "RUNNING", "worker_id": "w1:8083"}, "tasks": []any{}},
			},
			"my-sink": map[string]any{
				"info":   map[string]any{"name": "my-sink", "config": map[string]any{}, "type": "sink"},
				"status": map[string]any{"name": "my-sink", "connector": map[string]any{"state": "PAUSED", "worker_id": "w2:8083"}, "tasks": []any{}},
			},
		})
	}))
	defer srv.Close()

	client := NewClient("test-cluster", srv.URL)
	connectors, err := client.ListConnectors(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(connectors) != 2 {
		t.Fatalf("expected 2 connectors, got %d", len(connectors))
	}

	// Build a map for order-independent assertions.
	byName := make(map[string]ConnectorInfo)
	for _, c := range connectors {
		byName[c.Name] = c
	}

	src, ok := byName["my-source"]
	if !ok {
		t.Fatal("expected connector 'my-source' not found")
	}
	if src.Type != "source" {
		t.Errorf("expected type 'source', got %q", src.Type)
	}
	if src.State != "RUNNING" {
		t.Errorf("expected state 'RUNNING', got %q", src.State)
	}
	if src.WorkerID != "w1:8083" {
		t.Errorf("expected workerID 'w1:8083', got %q", src.WorkerID)
	}

	sink, ok := byName["my-sink"]
	if !ok {
		t.Fatal("expected connector 'my-sink' not found")
	}
	if sink.Type != "sink" {
		t.Errorf("expected type 'sink', got %q", sink.Type)
	}
	if sink.State != "PAUSED" {
		t.Errorf("expected state 'PAUSED', got %q", sink.State)
	}
	if sink.WorkerID != "w2:8083" {
		t.Errorf("expected workerID 'w2:8083', got %q", sink.WorkerID)
	}
}

func TestClient_ListConnectors_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connectors" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer srv.Close()

	client := NewClient("test-cluster", srv.URL)
	connectors, err := client.ListConnectors(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(connectors) != 0 {
		t.Fatalf("expected 0 connectors, got %d", len(connectors))
	}
}

func TestClient_GetConnector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/connectors/test-conn":
			json.NewEncoder(w).Encode(map[string]any{
				"name":   "test-conn",
				"config": map[string]string{"connector.class": "com.example.Source", "tasks.max": "1"},
				"type":   "source",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/connectors/test-conn/status":
			json.NewEncoder(w).Encode(map[string]any{
				"name":      "test-conn",
				"connector": map[string]any{"state": "RUNNING", "worker_id": "w1:8083"},
				"tasks": []map[string]any{
					{"id": 0, "state": "RUNNING", "worker_id": "w1:8083"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient("test-cluster", srv.URL)
	detail, err := client.GetConnector(context.Background(), "test-conn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if detail.Name != "test-conn" {
		t.Errorf("expected name 'test-conn', got %q", detail.Name)
	}
	if detail.Type != "source" {
		t.Errorf("expected type 'source', got %q", detail.Type)
	}
	if detail.State != "RUNNING" {
		t.Errorf("expected state 'RUNNING', got %q", detail.State)
	}
	if detail.WorkerID != "w1:8083" {
		t.Errorf("expected workerID 'w1:8083', got %q", detail.WorkerID)
	}
	if detail.Config["connector.class"] != "com.example.Source" {
		t.Errorf("expected connector.class 'com.example.Source', got %q", detail.Config["connector.class"])
	}
	if detail.Config["tasks.max"] != "1" {
		t.Errorf("expected tasks.max '1', got %q", detail.Config["tasks.max"])
	}
	if len(detail.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(detail.Tasks))
	}
	if detail.Tasks[0].ID != 0 {
		t.Errorf("expected task ID 0, got %d", detail.Tasks[0].ID)
	}
	if detail.Tasks[0].State != "RUNNING" {
		t.Errorf("expected task state 'RUNNING', got %q", detail.Tasks[0].State)
	}
	if detail.Tasks[0].WorkerID != "w1:8083" {
		t.Errorf("expected task workerID 'w1:8083', got %q", detail.Tasks[0].WorkerID)
	}
}

func TestClient_CreateConnector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/connectors":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("failed to decode request body: %v", err)
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			if body["name"] != "new-conn" {
				t.Errorf("expected name 'new-conn', got %v", body["name"])
			}
			json.NewEncoder(w).Encode(map[string]any{
				"name":   "new-conn",
				"config": map[string]string{"connector.class": "com.example.Sink"},
				"type":   "sink",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/connectors/new-conn":
			json.NewEncoder(w).Encode(map[string]any{
				"name":   "new-conn",
				"config": map[string]string{"connector.class": "com.example.Sink"},
				"type":   "sink",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/connectors/new-conn/status":
			json.NewEncoder(w).Encode(map[string]any{
				"name":      "new-conn",
				"connector": map[string]any{"state": "RUNNING", "worker_id": "w1:8083"},
				"tasks":     []any{},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient("test-cluster", srv.URL)
	detail, err := client.CreateConnector(context.Background(), CreateConnectorRequest{
		Name:   "new-conn",
		Config: map[string]string{"connector.class": "com.example.Sink"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Name != "new-conn" {
		t.Errorf("expected name 'new-conn', got %q", detail.Name)
	}
	if detail.Type != "sink" {
		t.Errorf("expected type 'sink', got %q", detail.Type)
	}
}

func TestClient_DeleteConnector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/connectors/test-conn" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient("test-cluster", srv.URL)
	err := client.DeleteConnector(context.Background(), "test-conn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_RestartConnector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/connectors/test-conn/restart" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient("test-cluster", srv.URL)
	err := client.RestartConnector(context.Background(), "test-conn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_PauseConnector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/connectors/test-conn/pause" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	client := NewClient("test-cluster", srv.URL)
	err := client.PauseConnector(context.Background(), "test-conn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_ResumeConnector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/connectors/test-conn/resume" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	client := NewClient("test-cluster", srv.URL)
	err := client.ResumeConnector(context.Background(), "test-conn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error_code":50001,"message":"Internal Server Error"}`))
	}))
	defer srv.Close()

	client := NewClient("test-cluster", srv.URL)
	_, err := client.ListConnectors(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "500") {
		t.Errorf("expected error to contain status code '500', got: %s", errMsg)
	}
}

func TestClient_ConnectClusterName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connectors" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"my-source": map[string]any{
				"info":   map[string]any{"name": "my-source", "config": map[string]any{}, "type": "source"},
				"status": map[string]any{"name": "my-source", "connector": map[string]any{"state": "RUNNING", "worker_id": "w1:8083"}, "tasks": []any{}},
			},
			"my-sink": map[string]any{
				"info":   map[string]any{"name": "my-sink", "config": map[string]any{}, "type": "sink"},
				"status": map[string]any{"name": "my-sink", "connector": map[string]any{"state": "RUNNING", "worker_id": "w2:8083"}, "tasks": []any{}},
			},
		})
	}))
	defer srv.Close()

	client := NewClient("prod-connect", srv.URL)
	connectors, err := client.ListConnectors(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(connectors) != 2 {
		t.Fatalf("expected 2 connectors, got %d", len(connectors))
	}

	for _, c := range connectors {
		if c.ConnectCluster != "prod-connect" {
			t.Errorf("expected ConnectCluster 'prod-connect' for connector %q, got %q", c.Name, c.ConnectCluster)
		}
	}
}

