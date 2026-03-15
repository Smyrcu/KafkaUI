package auth

import (
	"encoding/json"
	"testing"
)

// boolPtr is a helper to get a *bool from a bool literal.
func boolPtr(b bool) *bool { return &b }

func TestParseIdentityClaims(t *testing.T) {
	tests := []struct {
		name         string
		claims       map[string]any
		providerName string
		wantErr      bool
		check        func(t *testing.T, id *UserIdentity)
	}{
		{
			name:         "basic claims — sub, email, name, picture",
			providerName: "keycloak",
			claims: map[string]any{
				"sub":     "user-123",
				"email":   "alice@example.com",
				"name":    "Alice",
				"picture": "https://example.com/avatar.png",
			},
			check: func(t *testing.T, id *UserIdentity) {
				t.Helper()
				if id.ExternalID != "user-123" {
					t.Errorf("ExternalID = %q, want %q", id.ExternalID, "user-123")
				}
				if id.Email != "alice@example.com" {
					t.Errorf("Email = %q, want %q", id.Email, "alice@example.com")
				}
				if id.Name != "Alice" {
					t.Errorf("Name = %q, want %q", id.Name, "Alice")
				}
				if id.AvatarURL != "https://example.com/avatar.png" {
					t.Errorf("AvatarURL = %q, want %q", id.AvatarURL, "https://example.com/avatar.png")
				}
				if id.ProviderName != "keycloak" {
					t.Errorf("ProviderName = %q, want %q", id.ProviderName, "keycloak")
				}
				if id.ProviderType != "oidc" {
					t.Errorf("ProviderType = %q, want %q", id.ProviderType, "oidc")
				}
				if len(id.Orgs) != 0 {
					t.Errorf("Orgs = %v, want empty", id.Orgs)
				}
			},
		},
		{
			name:         "groups claim maps to Orgs",
			providerName: "google",
			claims: map[string]any{
				"sub":    "u1",
				"email":  "u@example.com",
				"groups": []string{"team-a", "team-b"},
			},
			check: func(t *testing.T, id *UserIdentity) {
				t.Helper()
				if len(id.Orgs) != 2 {
					t.Fatalf("len(Orgs) = %d, want 2", len(id.Orgs))
				}
				if id.Orgs[0] != "team-a" || id.Orgs[1] != "team-b" {
					t.Errorf("Orgs = %v, want [team-a team-b]", id.Orgs)
				}
			},
		},
		{
			name:         "realm_access.roles maps to Orgs (Keycloak)",
			providerName: "keycloak",
			claims: map[string]any{
				"sub":   "u2",
				"email": "u@example.com",
				"realm_access": map[string]any{
					"roles": []string{"admin", "viewer"},
				},
			},
			check: func(t *testing.T, id *UserIdentity) {
				t.Helper()
				if len(id.Orgs) != 2 {
					t.Fatalf("len(Orgs) = %d, want 2", len(id.Orgs))
				}
				if id.Orgs[0] != "admin" || id.Orgs[1] != "viewer" {
					t.Errorf("Orgs = %v, want [admin viewer]", id.Orgs)
				}
			},
		},
		{
			name:         "groups claim preferred over realm_access.roles",
			providerName: "keycloak",
			claims: map[string]any{
				"sub":    "u3",
				"email":  "u@example.com",
				"groups": []string{"from-groups"},
				"realm_access": map[string]any{
					"roles": []string{"from-realm-access"},
				},
			},
			check: func(t *testing.T, id *UserIdentity) {
				t.Helper()
				if len(id.Orgs) != 1 || id.Orgs[0] != "from-groups" {
					t.Errorf("Orgs = %v, want [from-groups]", id.Orgs)
				}
			},
		},
		{
			name:         "email_verified false clears email",
			providerName: "oidcp",
			claims: map[string]any{
				"sub":            "u4",
				"email":          "attacker@evil.com",
				"email_verified": false,
			},
			check: func(t *testing.T, id *UserIdentity) {
				t.Helper()
				if id.Email != "" {
					t.Errorf("Email = %q, want empty (email_verified=false)", id.Email)
				}
			},
		},
		{
			name:         "email_verified true keeps email",
			providerName: "oidcp",
			claims: map[string]any{
				"sub":            "u5",
				"email":          "legit@example.com",
				"email_verified": true,
			},
			check: func(t *testing.T, id *UserIdentity) {
				t.Helper()
				if id.Email != "legit@example.com" {
					t.Errorf("Email = %q, want %q", id.Email, "legit@example.com")
				}
			},
		},
		{
			name:         "no groups and no realm_access yields empty Orgs",
			providerName: "oidcp",
			claims: map[string]any{
				"sub":   "u6",
				"email": "plain@example.com",
			},
			check: func(t *testing.T, id *UserIdentity) {
				t.Helper()
				if len(id.Orgs) != 0 {
					t.Errorf("Orgs = %v, want empty", id.Orgs)
				}
			},
		},
		{
			name:         "missing email_verified field keeps email as-is",
			providerName: "oidcp",
			claims: map[string]any{
				"sub":   "u7",
				"email": "user@example.com",
				// no email_verified field
			},
			check: func(t *testing.T, id *UserIdentity) {
				t.Helper()
				if id.Email != "user@example.com" {
					t.Errorf("Email = %q, want %q", id.Email, "user@example.com")
				}
			},
		},
		{
			name:         "invalid JSON returns error",
			providerName: "oidcp",
			wantErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var claimsJSON []byte
			var err error

			if tc.claims == nil && tc.wantErr {
				// Use invalid JSON for the error case.
				claimsJSON = []byte("{invalid json}")
			} else {
				claimsJSON, err = json.Marshal(tc.claims)
				if err != nil {
					t.Fatalf("marshalling test claims: %v", err)
				}
			}

			id, err := parseIdentityClaims(claimsJSON, tc.providerName)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.check != nil {
				tc.check(t, id)
			}
		})
	}
}
