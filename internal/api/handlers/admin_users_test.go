package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/auth"
)

// newTestUserStore creates an in-memory UserStore for use in handler tests.
func newTestUserStore(t *testing.T) *auth.UserStore {
	t.Helper()
	store, err := auth.NewUserStore(":memory:")
	if err != nil {
		t.Fatalf("NewUserStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// mustUpsertUser creates a user in the store and fails the test on error.
func mustUpsertUser(t *testing.T, store *auth.UserStore, externalID, email, name string) *auth.User {
	t.Helper()
	identity := &auth.UserIdentity{
		ProviderName: "github",
		ProviderType: "oauth2",
		ExternalID:   externalID,
		Email:        email,
		Name:         name,
	}
	user, _, err := store.UpsertUser(identity)
	if err != nil {
		t.Fatalf("UpsertUser(%s): %v", externalID, err)
	}
	return user
}

// newAdminUsersRouter wires up the AdminUsersHandler on a chi router.
func newAdminUsersRouter(store *auth.UserStore) chi.Router {
	h := NewAdminUsersHandler(store)
	r := chi.NewRouter()
	r.Get("/admin/users", h.List)
	r.Get("/admin/users/{id}", h.Get)
	r.Put("/admin/users/{id}/roles", h.SetRoles)
	r.Delete("/admin/users/{id}", h.Delete)
	return r
}

func TestAdminUsers_List(t *testing.T) {
	store := newTestUserStore(t)
	mustUpsertUser(t, store, "ext-1", "alice@example.com", "Alice")
	mustUpsertUser(t, store, "ext-2", "bob@example.com", "Bob")

	r := newAdminUsersRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var users []auth.User
	if err := json.NewDecoder(rec.Body).Decode(&users); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

func TestAdminUsers_Get(t *testing.T) {
	store := newTestUserStore(t)
	created := mustUpsertUser(t, store, "ext-get", "charlie@example.com", "Charlie")

	r := newAdminUsersRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/"+created.ID, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var user auth.User
	if err := json.NewDecoder(rec.Body).Decode(&user); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if user.ID != created.ID {
		t.Errorf("expected ID %q, got %q", created.ID, user.ID)
	}
	if user.Email != "charlie@example.com" {
		t.Errorf("expected email %q, got %q", "charlie@example.com", user.Email)
	}
	if user.Name != "Charlie" {
		t.Errorf("expected name %q, got %q", "Charlie", user.Name)
	}
}

func TestAdminUsers_GetNotFound(t *testing.T) {
	store := newTestUserStore(t)
	r := newAdminUsersRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/nonexistent-id", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUsers_SetRoles(t *testing.T) {
	store := newTestUserStore(t)
	created := mustUpsertUser(t, store, "ext-roles", "dave@example.com", "Dave")

	r := newAdminUsersRouter(store)

	// Set roles for the user.
	body, _ := json.Marshal(map[string]any{"roles": []string{"admin", "viewer"}})
	req := httptest.NewRequest(http.MethodPut, "/admin/users/"+created.ID+"/roles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var result map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result["status"] != "roles updated" {
		t.Errorf("expected status 'roles updated', got %q", result["status"])
	}

	// Verify the user now has the expected roles.
	getReq := httptest.NewRequest(http.MethodGet, "/admin/users/"+created.ID, nil)
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, getReq)

	var user auth.User
	if err := json.NewDecoder(getRec.Body).Decode(&user); err != nil {
		t.Fatalf("decode user response: %v", err)
	}
	if len(user.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d: %v", len(user.Roles), user.Roles)
	}

	rolesSet := make(map[string]bool)
	for _, role := range user.Roles {
		rolesSet[role] = true
	}
	if !rolesSet["admin"] {
		t.Error("expected role 'admin' to be present")
	}
	if !rolesSet["viewer"] {
		t.Error("expected role 'viewer' to be present")
	}
}

func TestAdminUsers_SetRoles_NotFound(t *testing.T) {
	store := newTestUserStore(t)
	r := newAdminUsersRouter(store)

	body, _ := json.Marshal(map[string]any{"roles": []string{"admin"}})
	req := httptest.NewRequest(http.MethodPut, "/admin/users/nonexistent-id/roles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUsers_SetRoles_Validation(t *testing.T) {
	store := newTestUserStore(t)
	created := mustUpsertUser(t, store, "ext-val", "frank@example.com", "Frank")
	r := newAdminUsersRouter(store)

	send := func(roles []string) int {
		body, _ := json.Marshal(map[string]any{"roles": roles})
		req := httptest.NewRequest(http.MethodPut, "/admin/users/"+created.ID+"/roles", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}

	t.Run("empty role name", func(t *testing.T) {
		if code := send([]string{""}); code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", code)
		}
	})

	t.Run("role name too long", func(t *testing.T) {
		long := make([]byte, 65)
		for i := range long {
			long[i] = 'a'
		}
		if code := send([]string{string(long)}); code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", code)
		}
	})

	t.Run("invalid characters in role name", func(t *testing.T) {
		if code := send([]string{"bad role!"}); code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", code)
		}
	})

	t.Run("too many roles", func(t *testing.T) {
		roles := make([]string, 11)
		for i := range roles {
			roles[i] = fmt.Sprintf("role%d", i)
		}
		if code := send(roles); code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", code)
		}
	})

	t.Run("valid role names accepted", func(t *testing.T) {
		if code := send([]string{"admin", "viewer-2", "editor_role"}); code != http.StatusOK {
			t.Errorf("expected 200, got %d", code)
		}
	})
}

func TestAdminUsers_Delete(t *testing.T) {
	store := newTestUserStore(t)
	created := mustUpsertUser(t, store, "ext-del", "eve@example.com", "Eve")

	r := newAdminUsersRouter(store)

	// Delete the user.
	req := httptest.NewRequest(http.MethodDelete, "/admin/users/"+created.ID, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var result map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result["status"] != "user deleted" {
		t.Errorf("expected status 'user deleted', got %q", result["status"])
	}

	// Verify subsequent GET returns 404.
	getReq := httptest.NewRequest(http.MethodGet, "/admin/users/"+created.ID, nil)
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", getRec.Code)
	}
}
