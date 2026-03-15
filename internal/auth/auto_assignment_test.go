package auth

import (
	"testing"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

func TestAutoAssign_AuthenticatedMatchesAll(t *testing.T) {
	rules := []config.AutoAssignmentRule{
		{Role: "viewer", Match: config.AutoAssignmentMatch{Authenticated: true}},
	}

	identity := &UserIdentity{Email: "anyone@example.com"}
	roles := AutoAssign(rules, identity)

	if len(roles) != 1 || roles[0] != "viewer" {
		t.Errorf("expected [viewer], got %v", roles)
	}
}

func TestAutoAssign_AuthenticatedRequiresEmail(t *testing.T) {
	rules := []config.AutoAssignmentRule{
		{Role: "viewer", Match: config.AutoAssignmentMatch{Authenticated: true}},
	}

	identity := &UserIdentity{Email: ""}
	roles := AutoAssign(rules, identity)

	if len(roles) != 0 {
		t.Errorf("expected no roles for unauthenticated user, got %v", roles)
	}
}

func TestAutoAssign_EmailMatch(t *testing.T) {
	rules := []config.AutoAssignmentRule{
		{Role: "admin", Match: config.AutoAssignmentMatch{Emails: []string{"alice@example.com"}}},
	}

	t.Run("match", func(t *testing.T) {
		identity := &UserIdentity{Email: "alice@example.com"}
		roles := AutoAssign(rules, identity)
		if len(roles) != 1 || roles[0] != "admin" {
			t.Errorf("expected [admin], got %v", roles)
		}
	})

	t.Run("case-insensitive match", func(t *testing.T) {
		identity := &UserIdentity{Email: "Alice@Example.COM"}
		roles := AutoAssign(rules, identity)
		if len(roles) != 1 || roles[0] != "admin" {
			t.Errorf("expected [admin] for case-insensitive match, got %v", roles)
		}
	})

	t.Run("no match", func(t *testing.T) {
		identity := &UserIdentity{Email: "bob@example.com"}
		roles := AutoAssign(rules, identity)
		if len(roles) != 0 {
			t.Errorf("expected no roles for non-matching email, got %v", roles)
		}
	})
}

func TestAutoAssign_DomainMatch(t *testing.T) {
	rules := []config.AutoAssignmentRule{
		{Role: "staff", Match: config.AutoAssignmentMatch{EmailDomains: []string{"@company.com"}}},
	}

	t.Run("match", func(t *testing.T) {
		identity := &UserIdentity{Email: "user@company.com"}
		roles := AutoAssign(rules, identity)
		if len(roles) != 1 || roles[0] != "staff" {
			t.Errorf("expected [staff], got %v", roles)
		}
	})

	t.Run("no match different domain", func(t *testing.T) {
		identity := &UserIdentity{Email: "user@other.com"}
		roles := AutoAssign(rules, identity)
		if len(roles) != 0 {
			t.Errorf("expected no roles for different domain, got %v", roles)
		}
	})
}

func TestAutoAssign_GitHubOrgMatch(t *testing.T) {
	rules := []config.AutoAssignmentRule{
		{Role: "engineer", Match: config.AutoAssignmentMatch{GitHubOrgs: []string{"my-org"}}},
	}

	t.Run("org present", func(t *testing.T) {
		identity := &UserIdentity{Email: "dev@example.com", Orgs: []string{"other-org", "my-org"}}
		roles := AutoAssign(rules, identity)
		if len(roles) != 1 || roles[0] != "engineer" {
			t.Errorf("expected [engineer], got %v", roles)
		}
	})

	t.Run("org absent", func(t *testing.T) {
		identity := &UserIdentity{Email: "dev@example.com", Orgs: []string{"other-org"}}
		roles := AutoAssign(rules, identity)
		if len(roles) != 0 {
			t.Errorf("expected no roles when org not present, got %v", roles)
		}
	})
}

func TestAutoAssign_ANDLogicWithinMatch(t *testing.T) {
	rules := []config.AutoAssignmentRule{
		{Role: "trusted", Match: config.AutoAssignmentMatch{
			Emails:     []string{"alice@company.com"},
			GitHubOrgs: []string{"trusted-org"},
		}},
	}

	t.Run("both conditions met", func(t *testing.T) {
		identity := &UserIdentity{Email: "alice@company.com", Orgs: []string{"trusted-org"}}
		roles := AutoAssign(rules, identity)
		if len(roles) != 1 || roles[0] != "trusted" {
			t.Errorf("expected [trusted], got %v", roles)
		}
	})

	t.Run("only email matches", func(t *testing.T) {
		identity := &UserIdentity{Email: "alice@company.com", Orgs: []string{"other-org"}}
		roles := AutoAssign(rules, identity)
		if len(roles) != 0 {
			t.Errorf("expected no roles when only email matches, got %v", roles)
		}
	})

	t.Run("only org matches", func(t *testing.T) {
		identity := &UserIdentity{Email: "bob@company.com", Orgs: []string{"trusted-org"}}
		roles := AutoAssign(rules, identity)
		if len(roles) != 0 {
			t.Errorf("expected no roles when only org matches, got %v", roles)
		}
	})
}

func TestAutoAssign_ORLogicAcrossRules(t *testing.T) {
	rules := []config.AutoAssignmentRule{
		{Role: "viewer", Match: config.AutoAssignmentMatch{Authenticated: true}},
		{Role: "editor", Match: config.AutoAssignmentMatch{EmailDomains: []string{"@company.com"}}},
		{Role: "admin", Match: config.AutoAssignmentMatch{Emails: []string{"boss@company.com"}}},
	}

	t.Run("matches all rules", func(t *testing.T) {
		identity := &UserIdentity{Email: "boss@company.com"}
		roles := AutoAssign(rules, identity)
		if len(roles) != 3 {
			t.Errorf("expected 3 roles, got %v", roles)
		}
	})

	t.Run("matches two rules", func(t *testing.T) {
		identity := &UserIdentity{Email: "staff@company.com"}
		roles := AutoAssign(rules, identity)
		if len(roles) != 2 {
			t.Errorf("expected [viewer editor], got %v", roles)
		}
	})

	t.Run("matches one rule", func(t *testing.T) {
		identity := &UserIdentity{Email: "outsider@other.com"}
		roles := AutoAssign(rules, identity)
		if len(roles) != 1 || roles[0] != "viewer" {
			t.Errorf("expected [viewer], got %v", roles)
		}
	})
}

func TestAutoAssign_DeduplicatesRoles(t *testing.T) {
	rules := []config.AutoAssignmentRule{
		{Role: "viewer", Match: config.AutoAssignmentMatch{Authenticated: true}},
		{Role: "viewer", Match: config.AutoAssignmentMatch{EmailDomains: []string{"@company.com"}}},
		{Role: "editor", Match: config.AutoAssignmentMatch{Emails: []string{"alice@company.com"}}},
	}

	identity := &UserIdentity{Email: "alice@company.com"}
	roles := AutoAssign(rules, identity)

	if len(roles) != 2 {
		t.Errorf("expected 2 deduplicated roles, got %v", roles)
	}

	seen := make(map[string]int)
	for _, r := range roles {
		seen[r]++
	}
	for role, count := range seen {
		if count > 1 {
			t.Errorf("role %q appears %d times, expected 1", role, count)
		}
	}
}
