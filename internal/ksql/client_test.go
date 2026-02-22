package ksql

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_Execute_ShowStreams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/ksql" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{
				"@type":         "streams",
				"statementText": "SHOW STREAMS;",
				"streams": []map[string]string{
					{"name": "TEST_STREAM", "topic": "test-topic", "format": "JSON"},
				},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	result, err := client.Execute(context.Background(), "SHOW STREAMS;")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Type != "streams" {
		t.Errorf("expected type 'streams', got %q", result.Type)
	}
	if result.StatementText != "SHOW STREAMS;" {
		t.Errorf("expected statementText 'SHOW STREAMS;', got %q", result.StatementText)
	}
	if len(result.Data) == 0 {
		t.Error("expected non-empty Data")
	}
}

func TestClient_Execute_CreateStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/ksql" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{
				"@type":         "currentStatus",
				"statementText": "CREATE STREAM...",
				"commandStatus": map[string]string{
					"status":  "SUCCESS",
					"message": "Stream created",
				},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	result, err := client.Execute(context.Background(), "CREATE STREAM...")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Type != "currentStatus" {
		t.Errorf("expected type 'currentStatus', got %q", result.Type)
	}

	dataStr := string(result.Data)
	if !strings.Contains(dataStr, "SUCCESS") {
		t.Errorf("expected Data to contain 'SUCCESS', got %s", dataStr)
	}
}

func TestClient_Execute_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ksql" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	result, err := client.Execute(context.Background(), "SHOW STREAMS;")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Type != "" {
		t.Errorf("expected empty type, got %q", result.Type)
	}
	if len(result.Data) != 0 {
		t.Errorf("expected nil/empty Data, got %s", string(result.Data))
	}
}

func TestClient_Execute_ValidatesRequestBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ksql" {
			http.NotFound(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var reqBody map[string]any
		if err := json.Unmarshal(body, &reqBody); err != nil {
			t.Fatalf("failed to parse request body as JSON: %v", err)
		}

		if _, ok := reqBody["ksql"]; !ok {
			t.Error("expected request body to contain 'ksql' field")
		}
		if _, ok := reqBody["streamsProperties"]; !ok {
			t.Error("expected request body to contain 'streamsProperties' field")
		}

		ksqlVal, ok := reqBody["ksql"].(string)
		if !ok || ksqlVal != "SHOW STREAMS;" {
			t.Errorf("expected ksql field to be 'SHOW STREAMS;', got %v", reqBody["ksql"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{
				"@type":         "streams",
				"statementText": "SHOW STREAMS;",
				"streams":       []map[string]string{},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.Execute(context.Background(), "SHOW STREAMS;")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_Info(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/info" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"KsqlServerInfo": map[string]string{
				"version":        "0.29.0",
				"kafkaClusterId": "abc123",
				"ksqlServiceId":  "default_",
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	info, err := client.Info(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	serverInfo, ok := info["KsqlServerInfo"]
	if !ok {
		t.Fatal("expected 'KsqlServerInfo' key in response")
	}

	infoMap, ok := serverInfo.(map[string]any)
	if !ok {
		t.Fatalf("expected KsqlServerInfo to be a map, got %T", serverInfo)
	}

	if infoMap["version"] != "0.29.0" {
		t.Errorf("expected version '0.29.0', got %v", infoMap["version"])
	}
	if infoMap["kafkaClusterId"] != "abc123" {
		t.Errorf("expected kafkaClusterId 'abc123', got %v", infoMap["kafkaClusterId"])
	}
	if infoMap["ksqlServiceId"] != "default_" {
		t.Errorf("expected ksqlServiceId 'default_', got %v", infoMap["ksqlServiceId"])
	}
}

func TestClient_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"@type":"currentStatus","error_code":50000,"message":"Internal Server Error"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.Execute(context.Background(), "SHOW STREAMS;")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errMsg := err.Error()
	if !contains(errMsg, "500") {
		t.Errorf("expected error to contain status code '500', got: %s", errMsg)
	}
}

func TestNewClient_TrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If the trailing slash was not trimmed, the path would be "//info"
		// which would not match. This handler only responds to the correct path.
		if r.URL.Path != "/info" {
			t.Errorf("unexpected path %q, trailing slash may not have been trimmed", r.URL.Path)
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"KsqlServerInfo": map[string]string{
				"version": "0.29.0",
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL + "/")
	info, err := client.Info(context.Background())
	if err != nil {
		t.Fatalf("unexpected error (trailing slash may not be trimmed): %v", err)
	}
	if _, ok := info["KsqlServerInfo"]; !ok {
		t.Error("expected 'KsqlServerInfo' key in response")
	}
}

// contains checks if substr is present in s.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
