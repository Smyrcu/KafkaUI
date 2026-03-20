package auth

import (
	"testing"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

func newTestStore(t *testing.T) *UserStore {
	t.Helper()
	s, err := NewUserStore(":memory:")
	if err != nil {
		t.Fatalf("NewUserStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUserStore_UpsertAndGet(t *testing.T) {
	s := newTestStore(t)

	identity := &UserIdentity{
		ProviderName: "github",
		ProviderType: "oauth2",
		ExternalID:   "gh-123",
		Email:        "alice@example.com",
		Name:         "Alice",
		AvatarURL:    "https://example.com/avatar.png",
		Orgs:         []string{"acme"},
		Teams:        []string{"backend"},
	}

	// First upsert — should create.
	user, created, err := s.UpsertUser(identity)
	if err != nil {
		t.Fatalf("UpsertUser (create): %v", err)
	}
	if !created {
		t.Error("expected created=true on first insert")
	}
	if user.ID == "" {
		t.Error("expected non-empty ID")
	}
	if user.Email != "alice@example.com" {
		t.Errorf("email: got %q, want %q", user.Email, "alice@example.com")
	}
	firstID := user.ID

	// Second upsert with updated name — should update, not create.
	identity.Name = "Alice Updated"
	user2, created2, err := s.UpsertUser(identity)
	if err != nil {
		t.Fatalf("UpsertUser (update): %v", err)
	}
	if created2 {
		t.Error("expected created=false on second upsert")
	}
	if user2.ID != firstID {
		t.Errorf("ID changed: got %q, want %q", user2.ID, firstID)
	}
	if user2.Name != "Alice Updated" {
		t.Errorf("name not updated: got %q", user2.Name)
	}

	// GetUser by ID.
	got, err := s.GetUser(firstID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.Name != "Alice Updated" {
		t.Errorf("GetUser name: got %q", got.Name)
	}
}

func TestUserStore_RoleAssignment(t *testing.T) {
	s := newTestStore(t)

	identity := &UserIdentity{
		ProviderName: "github",
		ProviderType: "oauth2",
		ExternalID:   "gh-456",
		Email:        "bob@example.com",
		Name:         "Bob",
	}
	user, _, err := s.UpsertUser(identity)
	if err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}

	// Assign roles.
	if err := s.AssignRole(user.ID, "admin"); err != nil {
		t.Fatalf("AssignRole admin: %v", err)
	}
	if err := s.AssignRole(user.ID, "viewer"); err != nil {
		t.Fatalf("AssignRole viewer: %v", err)
	}

	roles, err := s.GetRoles(user.ID)
	if err != nil {
		t.Fatalf("GetRoles: %v", err)
	}
	if len(roles) != 2 {
		t.Errorf("expected 2 roles, got %d: %v", len(roles), roles)
	}

	// Duplicate assign — should not error.
	if err := s.AssignRole(user.ID, "admin"); err != nil {
		t.Errorf("duplicate AssignRole should not error: %v", err)
	}
	roles, _ = s.GetRoles(user.ID)
	if len(roles) != 2 {
		t.Errorf("expected still 2 roles after duplicate assign, got %d", len(roles))
	}

	// Remove a role.
	if err := s.RemoveRole(user.ID, "viewer"); err != nil {
		t.Fatalf("RemoveRole: %v", err)
	}
	roles, _ = s.GetRoles(user.ID)
	if len(roles) != 1 || roles[0] != "admin" {
		t.Errorf("expected [admin] after remove, got %v", roles)
	}
}

func TestUserStore_ListUsers(t *testing.T) {
	s := newTestStore(t)

	for _, id := range []struct{ extID, email, name string }{
		{"gh-1", "user1@example.com", "User One"},
		{"gh-2", "user2@example.com", "User Two"},
	} {
		_, _, err := s.UpsertUser(&UserIdentity{
			ProviderName: "github",
			ProviderType: "oauth2",
			ExternalID:   id.extID,
			Email:        id.email,
			Name:         id.name,
		})
		if err != nil {
			t.Fatalf("UpsertUser %s: %v", id.extID, err)
		}
	}

	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

func TestUserStore_DeleteUser(t *testing.T) {
	s := newTestStore(t)

	user, _, err := s.UpsertUser(&UserIdentity{
		ProviderName: "github",
		ProviderType: "oauth2",
		ExternalID:   "gh-del",
		Email:        "delete@example.com",
		Name:         "DeleteMe",
	})
	if err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}

	// Assign a role so cascade delete can be verified.
	if err := s.AssignRole(user.ID, "admin"); err != nil {
		t.Fatalf("AssignRole: %v", err)
	}

	// Delete the user.
	if err := s.DeleteUser(user.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	// GetUser should now return an error.
	_, err = s.GetUser(user.ID)
	if err == nil {
		t.Error("expected error from GetUser after delete, got nil")
	}

	// Roles should also be gone (cascade).
	roles, err := s.GetRoles(user.ID)
	if err != nil {
		t.Fatalf("GetRoles after delete: %v", err)
	}
	if len(roles) != 0 {
		t.Errorf("expected 0 roles after user delete, got %d", len(roles))
	}
}

func TestResolveRoles_AdminOverrideTakesPrecedence(t *testing.T) {
	store, _ := NewUserStore(":memory:")
	defer store.Close()

	user, _, _ := store.UpsertUser(&UserIdentity{ProviderName: "github", ExternalID: "1", Email: "a@co.com", Name: "A"})
	store.AssignRole(user.ID, "admin")

	rules := []config.AutoAssignmentRule{
		{Role: "viewer", Match: config.AutoAssignmentMatch{Authenticated: true}},
	}

	roles, err := ResolveRoles(store, user.ID, &UserIdentity{Email: "a@co.com"}, rules, "viewer")
	if err != nil {
		t.Fatalf("ResolveRoles() error: %v", err)
	}
	if len(roles) != 1 || roles[0] != "admin" {
		t.Errorf("expected [admin] override, got %v", roles)
	}
}

func TestResolveRoles_FallsBackToAutoAssign(t *testing.T) {
	store, _ := NewUserStore(":memory:")
	defer store.Close()

	user, _, _ := store.UpsertUser(&UserIdentity{ProviderName: "github", ExternalID: "2", Email: "b@company.com", Name: "B"})

	rules := []config.AutoAssignmentRule{
		{Role: "operator", Match: config.AutoAssignmentMatch{EmailDomains: []string{"@company.com"}}},
	}

	roles, err := ResolveRoles(store, user.ID, &UserIdentity{Email: "b@company.com"}, rules, "viewer")
	if err != nil {
		t.Fatalf("ResolveRoles() error: %v", err)
	}
	if len(roles) != 1 || roles[0] != "operator" {
		t.Errorf("expected [operator] via auto-assign, got %v", roles)
	}
}

func TestResolveRoles_FallsBackToDefault(t *testing.T) {
	store, _ := NewUserStore(":memory:")
	defer store.Close()

	user, _, _ := store.UpsertUser(&UserIdentity{ProviderName: "google", ExternalID: "3", Email: "c@other.com", Name: "C"})

	roles, err := ResolveRoles(store, user.ID, &UserIdentity{Email: "c@other.com"}, nil, "viewer")
	if err != nil {
		t.Fatalf("ResolveRoles() error: %v", err)
	}
	if len(roles) != 1 || roles[0] != "viewer" {
		t.Errorf("expected [viewer] default, got %v", roles)
	}
}

func TestResolveRoles_ErrorOnStoreFailure(t *testing.T) {
	store, _ := NewUserStore(":memory:")
	// Close the store to force a DB error on GetRoles.
	store.Close()

	_, err := ResolveRoles(store, "any-user-id", &UserIdentity{Email: "x@co.com"}, nil, "viewer")
	if err == nil {
		t.Fatal("expected error when store is closed, got nil")
	}
}
