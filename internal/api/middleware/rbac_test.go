package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/auth"
	"github.com/Smyrcu/KafkaUI/internal/config"
)

func newTestStore(t *testing.T) *auth.UserStore {
	t.Helper()
	store, err := auth.NewUserStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create user store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func newTestRBAC() *auth.RBAC {
	return auth.NewRBAC(config.RBACConfig{
		RoleGroups: map[string][]string{
			"view": {"view_topics", "view_consumers"},
			"edit": {"create_topics", "delete_topics", "view"},
		},
		Rules: []config.RBACRule{
			{Role: "admin", Clusters: []string{"*"}, Actions: []string{"*"}},
			{Role: "viewer", Clusters: []string{"*"}, Actions: []string{"view"}},
		},
	})
}

func buildRequest(t *testing.T, clusterName string, session *auth.SessionData) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/"+clusterName+"/topics", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", clusterName)
	req = req.WithContext(context.WithValue(
		context.WithValue(req.Context(), chi.RouteCtxKey, rctx),
		UserContextKey, session,
	))
	return req
}

// TestRequireAction_Allowed verifies that an admin user with a wildcard rule is
// permitted to perform view_topics on the "prod" cluster.
func TestRequireAction_Allowed(t *testing.T) {
	store := newTestStore(t)

	user, _, err := store.UpsertUser(&auth.UserIdentity{
		ProviderName: "basic",
		ExternalID:   "admin-1",
		Email:        "admin@co.com",
		Name:         "Admin",
	})
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	if err := store.AssignRole(user.ID, "admin"); err != nil {
		t.Fatalf("assign role: %v", err)
	}

	deps := RBACDeps{
		RBAC:        newTestRBAC(),
		Store:       store,
		AutoRules:   nil,
		DefaultRole: "",
	}

	session := &auth.SessionData{UserID: user.ID, Email: "admin@co.com", Name: "Admin"}
	req := buildRequest(t, "prod", session)
	rec := httptest.NewRecorder()

	handler := RequireAction(deps, "view_topics", true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin with wildcard rule, got %d", rec.Code)
	}
}

// TestRequireAction_Denied verifies that a viewer user is forbidden from
// performing delete_topics (only has view group actions).
func TestRequireAction_Denied(t *testing.T) {
	store := newTestStore(t)

	user, _, err := store.UpsertUser(&auth.UserIdentity{
		ProviderName: "basic",
		ExternalID:   "viewer-1",
		Email:        "viewer@co.com",
		Name:         "Viewer",
	})
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	if err := store.AssignRole(user.ID, "viewer"); err != nil {
		t.Fatalf("assign role: %v", err)
	}

	deps := RBACDeps{
		RBAC:        newTestRBAC(),
		Store:       store,
		AutoRules:   nil,
		DefaultRole: "",
	}

	session := &auth.SessionData{UserID: user.ID, Email: "viewer@co.com", Name: "Viewer"}
	req := buildRequest(t, "prod", session)
	rec := httptest.NewRecorder()

	handler := RequireAction(deps, "delete_topics", true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called when user lacks the required action")
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer trying delete_topics, got %d", rec.Code)
	}
}

// TestRequireAction_AuthDisabled verifies that the middleware passes through
// without any session or RBAC check when auth is disabled.
func TestRequireAction_AuthDisabled(t *testing.T) {
	deps := RBACDeps{
		RBAC:        newTestRBAC(),
		Store:       nil, // store is never accessed when auth is disabled
		AutoRules:   nil,
		DefaultRole: "",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/prod/topics", nil)
	rec := httptest.NewRecorder()

	handler := RequireAction(deps, "delete_topics", false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when auth disabled, got %d", rec.Code)
	}
}
