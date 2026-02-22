package schema

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_ListSubjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/subjects":
			json.NewEncoder(w).Encode([]string{"subject-1", "subject-2"})
		case "/subjects/subject-1/versions/latest":
			json.NewEncoder(w).Encode(map[string]any{
				"subject":    "subject-1",
				"version":    3,
				"id":         101,
				"schemaType": "AVRO",
			})
		case "/subjects/subject-2/versions/latest":
			json.NewEncoder(w).Encode(map[string]any{
				"subject":    "subject-2",
				"version":    1,
				"id":         202,
				"schemaType": "JSON",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	subjects, err := client.ListSubjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subjects) != 2 {
		t.Fatalf("expected 2 subjects, got %d", len(subjects))
	}

	if subjects[0].Subject != "subject-1" {
		t.Errorf("expected subject 'subject-1', got %q", subjects[0].Subject)
	}
	if subjects[0].LatestVersion != 3 {
		t.Errorf("expected latest version 3, got %d", subjects[0].LatestVersion)
	}
	if subjects[0].LatestSchemaID != 101 {
		t.Errorf("expected latest schema ID 101, got %d", subjects[0].LatestSchemaID)
	}
	if subjects[0].SchemaType != "AVRO" {
		t.Errorf("expected schema type 'AVRO', got %q", subjects[0].SchemaType)
	}

	if subjects[1].Subject != "subject-2" {
		t.Errorf("expected subject 'subject-2', got %q", subjects[1].Subject)
	}
	if subjects[1].LatestVersion != 1 {
		t.Errorf("expected latest version 1, got %d", subjects[1].LatestVersion)
	}
	if subjects[1].LatestSchemaID != 202 {
		t.Errorf("expected latest schema ID 202, got %d", subjects[1].LatestSchemaID)
	}
	if subjects[1].SchemaType != "JSON" {
		t.Errorf("expected schema type 'JSON', got %q", subjects[1].SchemaType)
	}
}

func TestClient_ListSubjects_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/subjects":
			json.NewEncoder(w).Encode([]string{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	subjects, err := client.ListSubjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(subjects) != 0 {
		t.Fatalf("expected 0 subjects, got %d", len(subjects))
	}
}

func TestClient_GetSubjectDetails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/subjects/test/versions":
			json.NewEncoder(w).Encode([]int{1, 2})
		case "/subjects/test/versions/1":
			json.NewEncoder(w).Encode(map[string]any{
				"subject":    "test",
				"id":         1,
				"version":    1,
				"schema":     `{"type":"record","name":"Test","fields":[{"name":"id","type":"int"}]}`,
				"schemaType": "AVRO",
			})
		case "/subjects/test/versions/2":
			json.NewEncoder(w).Encode(map[string]any{
				"subject":    "test",
				"id":         2,
				"version":    2,
				"schema":     `{"type":"record","name":"Test","fields":[{"name":"id","type":"int"},{"name":"name","type":"string"}]}`,
				"schemaType": "AVRO",
			})
		case "/config/test":
			json.NewEncoder(w).Encode(map[string]string{
				"compatibilityLevel": "BACKWARD",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	detail, err := client.GetSubjectDetails(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if detail.Subject != "test" {
		t.Errorf("expected subject 'test', got %q", detail.Subject)
	}
	if detail.Compatibility != "BACKWARD" {
		t.Errorf("expected compatibility 'BACKWARD', got %q", detail.Compatibility)
	}
	if len(detail.Versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(detail.Versions))
	}

	if detail.Versions[0].Version != 1 {
		t.Errorf("expected version 1, got %d", detail.Versions[0].Version)
	}
	if detail.Versions[0].ID != 1 {
		t.Errorf("expected ID 1, got %d", detail.Versions[0].ID)
	}
	if detail.Versions[0].SchemaType != "AVRO" {
		t.Errorf("expected schema type 'AVRO', got %q", detail.Versions[0].SchemaType)
	}
	if detail.Versions[0].Schema == "" {
		t.Error("expected non-empty schema for version 1")
	}

	if detail.Versions[1].Version != 2 {
		t.Errorf("expected version 2, got %d", detail.Versions[1].Version)
	}
	if detail.Versions[1].ID != 2 {
		t.Errorf("expected ID 2, got %d", detail.Versions[1].ID)
	}
}

func TestClient_GetSubjectDetails_GlobalCompatibility(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/subjects/test/versions":
			json.NewEncoder(w).Encode([]int{1})
		case "/subjects/test/versions/1":
			json.NewEncoder(w).Encode(map[string]any{
				"subject":    "test",
				"id":         10,
				"version":    1,
				"schema":     `{"type":"record","name":"Test","fields":[]}`,
				"schemaType": "AVRO",
			})
		case "/config/test":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error_code":40401,"message":"Subject not found"}`))
		case "/config":
			json.NewEncoder(w).Encode(map[string]string{
				"compatibilityLevel": "FULL",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	detail, err := client.GetSubjectDetails(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if detail.Compatibility != "FULL" {
		t.Errorf("expected compatibility 'FULL' (global fallback), got %q", detail.Compatibility)
	}
	if len(detail.Versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(detail.Versions))
	}
	if detail.Versions[0].ID != 10 {
		t.Errorf("expected ID 10, got %d", detail.Versions[0].ID)
	}
}

func TestClient_CreateSchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/subjects/new-subject/versions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if body["schemaType"] != "AVRO" {
			t.Errorf("expected schemaType 'AVRO', got %q", body["schemaType"])
		}
		if body["schema"] == "" {
			t.Error("expected non-empty schema in request body")
		}

		json.NewEncoder(w).Encode(map[string]int{"id": 42})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	resp, err := client.CreateSchema(context.Background(), CreateSchemaRequest{
		Subject:    "new-subject",
		Schema:     `{"type":"record","name":"NewSubject","fields":[]}`,
		SchemaType: "AVRO",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != 42 {
		t.Errorf("expected ID 42, got %d", resp.ID)
	}
}

func TestClient_DeleteSubject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/subjects/test" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}

		json.NewEncoder(w).Encode([]int{1, 2})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	err := client.DeleteSubject(context.Background(), "test")
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

	client := NewClient(srv.URL)
	_, err := client.ListSubjects(context.Background())
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
		// If the trailing slash was not trimmed, the path would be "//subjects"
		// which would not match. This handler only responds to the correct path.
		if r.URL.Path != "/subjects" {
			t.Errorf("unexpected path %q, trailing slash may not have been trimmed", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode([]string{})
	}))
	defer srv.Close()

	client := NewClient(srv.URL + "/")
	subjects, err := client.ListSubjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error (trailing slash may not be trimmed): %v", err)
	}
	if len(subjects) != 0 {
		t.Fatalf("expected 0 subjects, got %d", len(subjects))
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
