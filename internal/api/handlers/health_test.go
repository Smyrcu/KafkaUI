package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestHealthHandler_Liveness(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewHealthHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	h.Liveness(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", body["status"])
	}
}

func TestHealthHandler_Readiness(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewHealthHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	h.Readiness(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["status"] == nil {
		t.Fatal("expected 'status' field in response")
	}
	components, ok := body["components"].(map[string]any)
	if !ok {
		t.Fatal("expected 'components' field as object in response")
	}
	if components["kafka"] == nil {
		t.Fatal("expected 'kafka' component in response")
	}
}

func TestHealthHandler_Readiness_WithInclude(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewHealthHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/readyz?include=schema-registry,connect", nil)
	rec := httptest.NewRecorder()

	h.Readiness(rec, req)

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	components, ok := body["components"].(map[string]any)
	if !ok {
		t.Fatal("expected 'components' field as object in response")
	}
	if components["kafka"] == nil {
		t.Fatal("expected 'kafka' component in response")
	}
	if components["schema-registry"] == nil {
		t.Fatal("expected 'schema-registry' component in response")
	}
	srComponent, ok := components["schema-registry"].(map[string]any)
	if !ok {
		t.Fatal("expected 'schema-registry' component to be an object")
	}
	if srComponent["status"] != "not_configured" {
		t.Errorf("expected schema-registry status 'not_configured', got %q", srComponent["status"])
	}

	if components["connect"] == nil {
		t.Fatal("expected 'connect' component in response")
	}
	connectComponent, ok := components["connect"].(map[string]any)
	if !ok {
		t.Fatal("expected 'connect' component to be an object")
	}
	if connectComponent["status"] != "not_configured" {
		t.Errorf("expected connect status 'not_configured', got %q", connectComponent["status"])
	}
}

func TestHealthHandler_ServiceCheck(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewHealthHandler(reg)

	// Set up chi router context for {service} URL param
	r := chi.NewRouter()
	r.Get("/readyz/{service}", h.ServiceCheck)

	req := httptest.NewRequest(http.MethodGet, "/readyz/schema-registry", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["status"] != "not_configured" {
		t.Errorf("expected status 'not_configured', got %q", body["status"])
	}
}

func TestHealthHandler_ServiceCheck_Unknown(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewHealthHandler(reg)

	r := chi.NewRouter()
	r.Get("/readyz/{service}", h.ServiceCheck)

	req := httptest.NewRequest(http.MethodGet, "/readyz/unknown-service", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 for unknown service, got %d", rec.Code)
	}
}
