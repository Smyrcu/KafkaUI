package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"golang.org/x/oauth2"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

const (
	githubAuthorizeURL = "https://github.com/login/oauth/authorize"
	githubTokenURL     = "https://github.com/login/oauth/access_token"
	githubAPIBase      = "https://api.github.com"
)

// GitHubProvider implements IdentityProvider for GitHub OAuth2, fetching
// user profile, primary email, organization memberships, and team memberships.
type GitHubProvider struct {
	name         string
	oauth2Config oauth2.Config
	apiBase      string
}

// NewGitHubProvider creates a GitHubProvider from the given OAuth2 provider
// config and redirect URL. When apiBaseOverride is non-empty it is used as the
// base URL for both the OAuth2 endpoints and the REST API (useful in tests
// against a local httptest server).
func NewGitHubProvider(cfg config.OAuth2Provider, redirectURL string, apiBaseOverride string) *GitHubProvider {
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{"user:email", "read:org"}
	}

	apiBase := githubAPIBase
	tokenURL := githubTokenURL
	authorizeURL := githubAuthorizeURL

	if apiBaseOverride != "" {
		apiBase = apiBaseOverride + "/api/v3"
		tokenURL = apiBaseOverride + "/login/oauth/access_token"
		authorizeURL = apiBaseOverride + "/login/oauth/authorize"
	}

	return &GitHubProvider{
		name: cfg.Name,
		oauth2Config: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  redirectURL,
			Scopes:       scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  authorizeURL,
				TokenURL: tokenURL,
			},
		},
		apiBase: apiBase,
	}
}

// Name returns the provider's configured name (e.g. "github").
func (p *GitHubProvider) Name() string { return p.name }

// Type returns "oauth2" for all GitHub providers.
func (p *GitHubProvider) Type() string { return "oauth2" }

// AuthCodeURL returns the GitHub authorization redirect URL for the given state.
// nonce is accepted to satisfy the IdentityProvider interface but is not used
// because GitHub OAuth2 does not issue ID tokens with a nonce claim.
func (p *GitHubProvider) AuthCodeURL(state, _ string) string {
	return p.oauth2Config.AuthCodeURL(state)
}

// Exchange trades the authorization code for an access token, then fetches
// the user's profile, primary email, organization memberships, and team
// memberships from the GitHub API.
// expectedNonce is accepted to satisfy the IdentityProvider interface but is
// not verified because GitHub OAuth2 does not issue OIDC ID tokens.
func (p *GitHubProvider) Exchange(ctx context.Context, code, _ string) (*UserIdentity, error) {
	token, err := p.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchanging GitHub auth code: %w", err)
	}

	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

	user, err := p.fetchUser(client)
	if err != nil {
		return nil, err
	}

	email, err := p.fetchPrimaryEmail(client)
	if err != nil {
		return nil, err
	}

	orgs, _ := p.fetchOrgs(client)   // non-fatal
	teams, _ := p.fetchTeams(client) // non-fatal

	return &UserIdentity{
		ProviderName: p.name,
		ProviderType: "oauth2",
		ExternalID:   strconv.Itoa(user.ID),
		Email:        email,
		Name:         user.Name,
		AvatarURL:    user.AvatarURL,
		Orgs:         orgs,
		Teams:        teams,
	}, nil
}

// githubUser is the JSON shape of the /user GitHub API response.
type githubUser struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

func (p *GitHubProvider) fetchUser(client *http.Client) (*githubUser, error) {
	var user githubUser
	if err := p.getJSON(client, "/user", &user); err != nil {
		return nil, fmt.Errorf("fetching GitHub user: %w", err)
	}
	// Fall back to login when display name is not set.
	if user.Name == "" {
		user.Name = user.Login
	}
	return &user, nil
}

func (p *GitHubProvider) fetchPrimaryEmail(client *http.Client) (string, error) {
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := p.getJSON(client, "/user/emails", &emails); err != nil {
		return "", fmt.Errorf("fetching GitHub emails: %w", err)
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	return "", fmt.Errorf("no verified primary email found on GitHub account")
}

func (p *GitHubProvider) fetchOrgs(client *http.Client) ([]string, error) {
	var orgs []struct {
		Login string `json:"login"`
	}
	if err := p.getJSON(client, "/user/orgs", &orgs); err != nil {
		return nil, nil
	}
	result := make([]string, len(orgs))
	for i, o := range orgs {
		result[i] = o.Login
	}
	return result, nil
}

func (p *GitHubProvider) fetchTeams(client *http.Client) ([]string, error) {
	var teams []struct {
		Slug string `json:"slug"`
		Org  struct {
			Login string `json:"login"`
		} `json:"organization"`
	}
	if err := p.getJSON(client, "/user/teams", &teams); err != nil {
		return nil, nil
	}
	result := make([]string, len(teams))
	for i, t := range teams {
		result[i] = t.Org.Login + "/" + t.Slug
	}
	return result, nil
}

func (p *GitHubProvider) getJSON(client *http.Client, path string, dest any) error {
	resp, err := client.Get(p.apiBase + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API %s returned %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}
