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

	identity, err := auth.Authenticate("admin", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity.Name != "admin" {
		t.Errorf("expected name 'admin', got %q", identity.Name)
	}
	if identity.Email != "admin" {
		t.Errorf("expected email 'admin', got %q", identity.Email)
	}
	if identity.ProviderName != "basic" {
		t.Errorf("expected providerName 'basic', got %q", identity.ProviderName)
	}
	if identity.ProviderType != "basic" {
		t.Errorf("expected providerType 'basic', got %q", identity.ProviderType)
	}
	if identity.ExternalID != "admin" {
		t.Errorf("expected externalID 'admin', got %q", identity.ExternalID)
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

	identity, err := auth.Authenticate("viewer", "viewerpass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity.Name != "viewer" {
		t.Errorf("expected name 'viewer', got %q", identity.Name)
	}
	if identity.ExternalID != "viewer" {
		t.Errorf("expected externalID 'viewer', got %q", identity.ExternalID)
	}
}

func TestBasicAuthenticator_EmptyUsers(t *testing.T) {
	auth := NewBasicAuthenticator(nil)

	_, err := auth.Authenticate("anyone", "anything")
	if err == nil {
		t.Fatal("expected error with no users configured")
	}
}
