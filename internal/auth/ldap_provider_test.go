package auth

import (
	"log/slog"
	"os"
	"testing"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

func TestNewLDAPAuthenticator(t *testing.T) {
	cfg := config.LDAPConfig{
		URL:        "ldap://localhost:389",
		BindDN:     "cn=admin,dc=test,dc=com",
		SearchBase: "ou=people,dc=test,dc=com",
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	a := NewLDAPAuthenticator(cfg, logger)
	if a == nil {
		t.Fatal("expected non-nil authenticator")
	}
}

func TestLDAPAuthenticator_EmptyCredentials(t *testing.T) {
	cfg := config.LDAPConfig{
		URL:        "ldap://localhost:1",
		BindDN:     "cn=admin,dc=test,dc=com",
		SearchBase: "ou=people,dc=test,dc=com",
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	a := NewLDAPAuthenticator(cfg, logger)

	// Empty password should still attempt LDAP (and fail gracefully)
	_, err := a.Authenticate("user", "")
	if err == nil {
		t.Error("expected error for empty password")
	}
}

func TestLDAPAuthenticator_UnreachableServer(t *testing.T) {
	cfg := config.LDAPConfig{
		URL:               "ldap://192.0.2.1:389", // non-routable IP
		ConnectionTimeout: "1s",
		BindDN:            "cn=admin,dc=test,dc=com",
		SearchBase:        "ou=people,dc=test,dc=com",
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	a := NewLDAPAuthenticator(cfg, logger)

	_, err := a.Authenticate("user", "pass")
	if err == nil {
		t.Error("expected error for unreachable server")
	}
	if err.Error() != "invalid credentials" {
		t.Errorf("expected generic error, got: %v", err)
	}
}

func TestLDAPConfig_ConnectionTimeoutDuration(t *testing.T) {
	t.Run("empty returns 10s", func(t *testing.T) {
		cfg := config.LDAPConfig{}
		d := cfg.ConnectionTimeoutDuration()
		if d.Seconds() != 10 {
			t.Errorf("expected 10s, got %v", d)
		}
	})

	t.Run("valid duration parsed", func(t *testing.T) {
		cfg := config.LDAPConfig{ConnectionTimeout: "5s"}
		d := cfg.ConnectionTimeoutDuration()
		if d.Seconds() != 5 {
			t.Errorf("expected 5s, got %v", d)
		}
	})

	t.Run("invalid returns 10s default", func(t *testing.T) {
		cfg := config.LDAPConfig{ConnectionTimeout: "invalid"}
		d := cfg.ConnectionTimeoutDuration()
		if d.Seconds() != 10 {
			t.Errorf("expected 10s default, got %v", d)
		}
	})
}

func TestLDAPAutoAssignment_LDAPGroups(t *testing.T) {
	rules := []config.AutoAssignmentRule{
		{Role: "admin", Match: config.AutoAssignmentMatch{
			LDAPGroups: []string{"cn=kafka-admins,ou=groups,dc=test,dc=com"},
		}},
	}

	t.Run("matching group", func(t *testing.T) {
		identity := &UserIdentity{
			ExternalID: "uid=user,ou=people,dc=test,dc=com",
			Orgs:       []string{"cn=kafka-admins,ou=groups,dc=test,dc=com"},
		}
		roles := AutoAssign(rules, identity)
		if len(roles) != 1 || roles[0] != "admin" {
			t.Errorf("expected [admin], got %v", roles)
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		identity := &UserIdentity{
			ExternalID: "uid=user,ou=people,dc=test,dc=com",
			Orgs:       []string{"CN=Kafka-Admins,OU=Groups,DC=Test,DC=Com"},
		}
		roles := AutoAssign(rules, identity)
		if len(roles) != 1 || roles[0] != "admin" {
			t.Errorf("expected [admin] for case-insensitive match, got %v", roles)
		}
	})

	t.Run("no matching group", func(t *testing.T) {
		identity := &UserIdentity{
			ExternalID: "uid=user,ou=people,dc=test,dc=com",
			Orgs:       []string{"cn=viewers,ou=groups,dc=test,dc=com"},
		}
		roles := AutoAssign(rules, identity)
		if len(roles) != 0 {
			t.Errorf("expected no roles, got %v", roles)
		}
	})
}
