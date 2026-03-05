package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

func TestStatus_AuthDisabled(t *testing.T) {
	h := NewAuthHandler(nil, nil, nil, nil, nil, nil, false, nil)

	req := httptest.NewRequest(http.MethodGet, "/auth/status", nil)
	rr := httptest.NewRecorder()

	h.Status(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["enabled"] != false {
		t.Errorf("expected enabled=false, got %v", resp["enabled"])
	}
	if resp["types"] != nil {
		t.Errorf("expected types=nil, got %v", resp["types"])
	}
}

func TestStatus_BasicOnly(t *testing.T) {
	h := NewAuthHandler(nil, nil, nil, nil, nil, nil, true, []string{"basic"})

	req := httptest.NewRequest(http.MethodGet, "/auth/status", nil)
	rr := httptest.NewRecorder()

	h.Status(rr, req)

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", resp["enabled"])
	}
	types, ok := resp["types"].([]any)
	if !ok || len(types) != 1 || types[0] != "basic" {
		t.Errorf("expected types=[basic], got %v", resp["types"])
	}
}

func TestStatus_MultiAuth(t *testing.T) {
	h := NewAuthHandler(nil, nil, nil, nil, nil, nil, true, []string{"basic", "oidc"})

	req := httptest.NewRequest(http.MethodGet, "/auth/status", nil)
	rr := httptest.NewRecorder()

	h.Status(rr, req)

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)

	types, ok := resp["types"].([]any)
	if !ok || len(types) != 2 {
		t.Fatalf("expected types=[basic,oidc], got %v", resp["types"])
	}
	if types[0] != "basic" || types[1] != "oidc" {
		t.Errorf("expected [basic, oidc], got %v", types)
	}
}

func TestLoginBasic_NotEnabled(t *testing.T) {
	h := NewAuthHandler(nil, nil, nil, nil, nil, nil, true, []string{"oidc"})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	rr := httptest.NewRecorder()

	h.LoginBasic(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 when basic not in types, got %d", rr.Code)
	}
}

func TestStatus_MultiAuthWithProviders(t *testing.T) {
	providerCfg := []config.OIDCProvider{
		{Name: "google"},
		{Name: "auth0", DisplayName: "Auth0"},
	}
	h := NewAuthHandler(nil, providerCfg, nil, nil, nil, nil, true, []string{"basic", "oidc"})

	req := httptest.NewRequest(http.MethodGet, "/auth/status", nil)
	rr := httptest.NewRecorder()

	h.Status(rr, req)

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)

	types, ok := resp["types"].([]any)
	if !ok || len(types) != 2 {
		t.Fatalf("expected types=[basic,oidc], got %v", resp["types"])
	}

	providers, ok := resp["providers"].([]any)
	if !ok || len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %v", resp["providers"])
	}

	p0 := providers[0].(map[string]any)
	if p0["name"] != "google" {
		t.Errorf("expected first provider 'google', got %v", p0["name"])
	}

	p1 := providers[1].(map[string]any)
	if p1["name"] != "auth0" {
		t.Errorf("expected second provider 'auth0', got %v", p1["name"])
	}
	if p1["displayName"] != "Auth0" {
		t.Errorf("expected displayName 'Auth0', got %v", p1["displayName"])
	}
}

func TestLoginOIDC_NotEnabled(t *testing.T) {
	h := NewAuthHandler(nil, nil, nil, nil, nil, nil, true, []string{"basic"})

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rr := httptest.NewRecorder()

	h.LoginOIDC(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 when oidc not in types, got %d", rr.Code)
	}
}
