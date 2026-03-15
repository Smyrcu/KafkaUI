package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

// newGitHubTestServer sets up a minimal mock of the GitHub OAuth2 and REST API.
func newGitHubTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// OAuth2 token endpoint.
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": "test-token",
			"token_type":   "bearer",
		})
	})

	// GitHub REST API — user profile.
	mux.HandleFunc("/api/v3/user", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         12345,
			"login":      "alice",
			"name":       "Alice",
			"avatar_url": "https://avatar.url",
		})
	})

	// GitHub REST API — user emails.
	mux.HandleFunc("/api/v3/user/emails", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"email": "alice@example.com", "primary": true, "verified": true},
		})
	})

	// GitHub REST API — user orgs.
	mux.HandleFunc("/api/v3/user/orgs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"login": "my-org"},
			{"login": "other-org"},
		})
	})

	// GitHub REST API — user teams.
	mux.HandleFunc("/api/v3/user/teams", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"slug":         "team-a",
				"organization": map[string]string{"login": "my-org"},
			},
		})
	})

	return httptest.NewServer(mux)
}

func TestGitHubProvider_Exchange(t *testing.T) {
	srv := newGitHubTestServer(t)
	defer srv.Close()

	cfg := config.OAuth2Provider{
		Name:         "github",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}

	provider := NewGitHubProvider(cfg, "http://localhost/callback", srv.URL)

	identity, err := provider.Exchange(context.Background(), "dummy-code")
	if err != nil {
		t.Fatalf("Exchange() error: %v", err)
	}

	if identity.ProviderName != "github" {
		t.Errorf("ProviderName = %q, want %q", identity.ProviderName, "github")
	}
	if identity.ExternalID != "12345" {
		t.Errorf("ExternalID = %q, want %q", identity.ExternalID, "12345")
	}
	if identity.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", identity.Email, "alice@example.com")
	}
	if identity.Name != "Alice" {
		t.Errorf("Name = %q, want %q", identity.Name, "Alice")
	}
	if len(identity.Orgs) != 2 {
		t.Errorf("len(Orgs) = %d, want 2", len(identity.Orgs))
	}
	if len(identity.Teams) != 1 || identity.Teams[0] != "my-org/team-a" {
		t.Errorf("Teams = %v, want [my-org/team-a]", identity.Teams)
	}
}

func TestGitHubProvider_NameAndType(t *testing.T) {
	cfg := config.OAuth2Provider{
		Name:         "github",
		ClientID:     "cid",
		ClientSecret: "csec",
	}
	provider := NewGitHubProvider(cfg, "http://localhost/callback", "")

	if got := provider.Name(); got != "github" {
		t.Errorf("Name() = %q, want %q", got, "github")
	}
	if got := provider.Type(); got != "oauth2" {
		t.Errorf("Type() = %q, want %q", got, "oauth2")
	}
}
