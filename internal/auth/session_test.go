package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSessionManager_CreateAndGet(t *testing.T) {
	sm := NewSessionManager("test-secret-key-32-chars-long!!", 3600)

	data := SessionData{
		Token: "test-token",
		Email: "user@example.com",
		Name:  "Test User",
		Roles: []string{"admin"},
	}

	// Create a response recorder to capture the cookie
	rec := httptest.NewRecorder()
	if err := sm.CreateSession(rec, httptest.NewRequest(http.MethodGet, "/", nil), data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Extract cookie and put it in a new request
	cookies := rec.Result().Cookies()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}

	// Get session
	got, err := sm.GetSession(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Token != data.Token {
		t.Errorf("expected token %q, got %q", data.Token, got.Token)
	}
	if got.Email != data.Email {
		t.Errorf("expected email %q, got %q", data.Email, got.Email)
	}
	if got.Name != data.Name {
		t.Errorf("expected name %q, got %q", data.Name, got.Name)
	}
	if len(got.Roles) != len(data.Roles) {
		t.Fatalf("expected %d roles, got %d", len(data.Roles), len(got.Roles))
	}
	for i, role := range data.Roles {
		if got.Roles[i] != role {
			t.Errorf("expected role[%d] %q, got %q", i, role, got.Roles[i])
		}
	}
}

func TestSessionManager_InvalidSignature(t *testing.T) {
	sm := NewSessionManager("test-secret-key-32-chars-long!!", 3600)

	data := SessionData{
		Token: "test-token",
		Email: "user@example.com",
		Name:  "Test User",
		Roles: []string{"admin"},
	}

	rec := httptest.NewRecorder()
	if err := sm.CreateSession(rec, httptest.NewRequest(http.MethodGet, "/", nil), data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected at least one cookie")
	}

	// Tamper with the cookie value
	tampered := cookies[0]
	tampered.Value = tampered.Value + "tampered"

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(tampered)

	_, err := sm.GetSession(req)
	if err == nil {
		t.Fatal("expected error for tampered cookie, got nil")
	}
}

func TestSessionManager_NoCookie(t *testing.T) {
	sm := NewSessionManager("test-secret-key-32-chars-long!!", 3600)

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, err := sm.GetSession(req)
	if err == nil {
		t.Fatal("expected error for missing cookie, got nil")
	}
}

func TestSessionManager_ClearSession(t *testing.T) {
	sm := NewSessionManager("test-secret-key-32-chars-long!!", 3600)

	data := SessionData{
		Token: "test-token",
		Email: "user@example.com",
		Name:  "Test User",
		Roles: []string{"viewer"},
	}

	// Create a session
	createRec := httptest.NewRecorder()
	if err := sm.CreateSession(createRec, httptest.NewRequest(http.MethodGet, "/", nil), data); err != nil {
		t.Fatalf("unexpected error creating session: %v", err)
	}

	// Clear the session
	clearRec := httptest.NewRecorder()
	sm.ClearSession(clearRec, httptest.NewRequest(http.MethodGet, "/", nil))

	// Verify the cookie is expired
	cookies := clearRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected a cookie to be set when clearing session")
	}

	found := false
	for _, c := range cookies {
		if c.Name == "kafkaui_session" {
			found = true
			if c.MaxAge >= 0 {
				// An expired cookie should have MaxAge < 0 (or be set in the past)
				// Some implementations use MaxAge = -1
				if c.MaxAge > 0 {
					t.Errorf("expected expired cookie (MaxAge <= 0), got MaxAge=%d", c.MaxAge)
				}
			}
			if c.Value != "" {
				t.Errorf("expected empty cookie value, got %q", c.Value)
			}
			break
		}
	}
	if !found {
		t.Error("expected kafkaui_session cookie in clear response")
	}

	// Verify that using the cleared cookie does not return a valid session
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	_, err := sm.GetSession(req)
	if err == nil {
		t.Error("expected error after clearing session, got nil")
	}
}

func TestSessionManager_DefaultMaxAge(t *testing.T) {
	// Create with maxAge=0, should default to 86400
	sm := NewSessionManager("test-secret-key-32-chars-long!!", 0)

	data := SessionData{
		Token: "default-age-token",
		Email: "default@example.com",
		Name:  "Default User",
		Roles: []string{"viewer"},
	}

	rec := httptest.NewRecorder()
	if err := sm.CreateSession(rec, httptest.NewRequest(http.MethodGet, "/", nil), data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected at least one cookie")
	}

	found := false
	for _, c := range cookies {
		if c.Name == "kafkaui_session" {
			found = true
			if c.MaxAge != 86400 {
				t.Errorf("expected default MaxAge 86400, got %d", c.MaxAge)
			}
			break
		}
	}
	if !found {
		// Check Set-Cookie header directly as fallback
		setCookie := rec.Header().Get("Set-Cookie")
		if !strings.Contains(setCookie, "Max-Age=86400") {
			t.Errorf("expected Max-Age=86400 in Set-Cookie header, got %q", setCookie)
		}
	}

	// Verify the session is still readable
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	got, err := sm.GetSession(req)
	if err != nil {
		t.Fatalf("unexpected error reading session: %v", err)
	}
	if got.Email != data.Email {
		t.Errorf("expected email %q, got %q", data.Email, got.Email)
	}
}
