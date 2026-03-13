package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Smyrcu/KafkaUI/internal/auth"
)

func TestAuth_Disabled_PassesThrough(t *testing.T) {
	sm := auth.NewSessionManager("test-secret", 3600)

	handler := Auth(sm, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify no user context is set when auth is disabled
		user := r.Context().Value(UserContextKey)
		if user != nil {
			t.Error("expected no user context when auth is disabled")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestAuth_Enabled_NoCookie_Returns401(t *testing.T) {
	sm := auth.NewSessionManager("test-secret", 3600)

	handler := Auth(sm, true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called when no cookie is present")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestAuth_Enabled_ValidSession_PassesThrough(t *testing.T) {
	sm := auth.NewSessionManager("test-secret", 3600)

	// Create a session by recording the Set-Cookie from CreateSession.
	sessionData := auth.SessionData{
		Token: "test-token",
		Email: "alice@example.com",
		Name:  "Alice",
		Roles: []string{"admin"},
	}

	// Use a recorder to capture the session cookie set by CreateSession.
	cookieReq := httptest.NewRequest(http.MethodGet, "/", nil)
	cookieReq.Host = "localhost"
	cookieRec := httptest.NewRecorder()
	if err := sm.CreateSession(cookieRec, cookieReq, sessionData); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Extract the cookie from the response.
	cookies := cookieRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie to be set")
	}

	handler := Auth(sm, true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := r.Context().Value(UserContextKey)
		if user == nil {
			t.Fatal("expected user context to be set")
		}
		sd, ok := user.(*auth.SessionData)
		if !ok {
			t.Fatalf("expected *auth.SessionData, got %T", user)
		}
		if sd.Email != "alice@example.com" {
			t.Errorf("expected email 'alice@example.com', got %q", sd.Email)
		}
		if sd.Name != "Alice" {
			t.Errorf("expected name 'Alice', got %q", sd.Name)
		}
		if len(sd.Roles) != 1 || sd.Roles[0] != "admin" {
			t.Errorf("expected roles [admin], got %v", sd.Roles)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestAuth_Enabled_InvalidCookie_Returns401(t *testing.T) {
	sm := auth.NewSessionManager("test-secret", 3600)

	handler := Auth(sm, true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called with invalid cookie")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	req.AddCookie(&http.Cookie{
		Name:  "kafkaui_session",
		Value: "invalid-cookie-value",
	})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestAuth_Enabled_TamperedCookie_Returns401(t *testing.T) {
	// Create a session with one secret
	sm1 := auth.NewSessionManager("secret-one", 3600)

	sessionData := auth.SessionData{
		Token: "test-token",
		Email: "alice@example.com",
		Name:  "Alice",
	}

	cookieReq := httptest.NewRequest(http.MethodGet, "/", nil)
	cookieReq.Host = "localhost"
	cookieRec := httptest.NewRecorder()
	if err := sm1.CreateSession(cookieRec, cookieReq, sessionData); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	cookies := cookieRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie to be set")
	}

	// Verify with a different secret - should fail
	sm2 := auth.NewSessionManager("secret-two", 3600)

	handler := Auth(sm2, true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called with tampered cookie")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}
