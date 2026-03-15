package middleware

import (
	"context"
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
		UserID: "user-123",
		Email:  "alice@example.com",
		Name:   "Alice",
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
		if sd.UserID != "user-123" {
			t.Errorf("expected user_id 'user-123', got %q", sd.UserID)
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
		UserID: "user-123",
		Email:  "alice@example.com",
		Name:   "Alice",
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

func TestRequireRole_Disabled_PassesThrough(t *testing.T) {
	handler := RequireRole("admin", false, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when auth disabled, got %d", rec.Code)
	}
}

func TestRequireRole_NoSession_Returns401(t *testing.T) {
	handler := RequireRole("admin", true, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called without session")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireRole_NilStore_Returns403(t *testing.T) {
	session := &auth.SessionData{UserID: "user-123", Email: "test@test.com", Name: "Test"}
	ctx := context.WithValue(context.Background(), UserContextKey, session)

	handler := RequireRole("admin", true, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called with nil store")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRequireRole_UserHasRole_PassesThrough(t *testing.T) {
	store, err := auth.NewUserStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create user store: %v", err)
	}
	defer store.Close()

	// Create a user and assign the admin role
	user, _, err := store.UpsertUser(&auth.UserIdentity{
		ProviderName: "basic",
		ExternalID:   "admin-user",
		Email:        "admin@test.com",
		Name:         "Admin",
	})
	if err != nil {
		t.Fatalf("failed to upsert user: %v", err)
	}
	if err := store.AssignRole(user.ID, "admin"); err != nil {
		t.Fatalf("failed to assign role: %v", err)
	}

	session := &auth.SessionData{UserID: user.ID, Email: user.Email, Name: user.Name}
	ctx := context.WithValue(context.Background(), UserContextKey, session)

	handler := RequireRole("admin", true, store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRequireRole_UserLacksRole_Returns403(t *testing.T) {
	store, err := auth.NewUserStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create user store: %v", err)
	}
	defer store.Close()

	// Create a user with "viewer" role only
	user, _, err := store.UpsertUser(&auth.UserIdentity{
		ProviderName: "basic",
		ExternalID:   "viewer-user",
		Email:        "viewer@test.com",
		Name:         "Viewer",
	})
	if err != nil {
		t.Fatalf("failed to upsert user: %v", err)
	}
	if err := store.AssignRole(user.ID, "viewer"); err != nil {
		t.Fatalf("failed to assign role: %v", err)
	}

	session := &auth.SessionData{UserID: user.ID, Email: user.Email, Name: user.Name}
	ctx := context.WithValue(context.Background(), UserContextKey, session)

	handler := RequireRole("admin", true, store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called when user lacks required role")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}
