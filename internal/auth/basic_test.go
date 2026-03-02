package auth

import (
	"testing"

	"github.com/Smyrcu/KafkaUI/internal/config"
	"golang.org/x/crypto/bcrypt"
)

func hashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return string(hash)
}

func TestBasicAuthenticator_ValidCredentials(t *testing.T) {
	auth := NewBasicAuthenticator([]config.BasicUser{
		{Username: "admin", Password: hashPassword(t, "secret"), Roles: []string{"admin"}},
	})

	session, err := auth.Authenticate("admin", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.Name != "admin" {
		t.Errorf("expected name 'admin', got %q", session.Name)
	}
	if session.Email != "admin" {
		t.Errorf("expected email 'admin', got %q", session.Email)
	}
	if len(session.Roles) != 1 || session.Roles[0] != "admin" {
		t.Errorf("expected roles [admin], got %v", session.Roles)
	}
}

func TestBasicAuthenticator_WrongPassword(t *testing.T) {
	auth := NewBasicAuthenticator([]config.BasicUser{
		{Username: "admin", Password: hashPassword(t, "secret"), Roles: []string{"admin"}},
	})

	_, err := auth.Authenticate("admin", "wrong")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestBasicAuthenticator_UnknownUser(t *testing.T) {
	auth := NewBasicAuthenticator([]config.BasicUser{
		{Username: "admin", Password: hashPassword(t, "secret"), Roles: []string{"admin"}},
	})

	_, err := auth.Authenticate("unknown", "secret")
	if err == nil {
		t.Fatal("expected error for unknown user")
	}
}

func TestBasicAuthenticator_MultipleUsers(t *testing.T) {
	auth := NewBasicAuthenticator([]config.BasicUser{
		{Username: "admin", Password: hashPassword(t, "adminpass"), Roles: []string{"admin"}},
		{Username: "viewer", Password: hashPassword(t, "viewerpass"), Roles: []string{"viewer"}},
	})

	session, err := auth.Authenticate("viewer", "viewerpass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.Name != "viewer" {
		t.Errorf("expected name 'viewer', got %q", session.Name)
	}
	if len(session.Roles) != 1 || session.Roles[0] != "viewer" {
		t.Errorf("expected roles [viewer], got %v", session.Roles)
	}
}

func TestBasicAuthenticator_EmptyUsers(t *testing.T) {
	auth := NewBasicAuthenticator(nil)

	_, err := auth.Authenticate("anyone", "anything")
	if err == nil {
		t.Fatal("expected error with no users configured")
	}
}
