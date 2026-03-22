package auth

import "context"

// IdentityProvider abstracts OIDC and OAuth2 authentication flows.
type IdentityProvider interface {
	Name() string
	Type() string // "oidc" | "oauth2"
	// AuthCodeURL builds the provider redirect URL. nonce is an opaque
	// value that OIDC providers embed in the ID token to prevent replay
	// attacks; OAuth2-only providers (e.g. GitHub) may ignore it.
	AuthCodeURL(state, nonce string) string
	// Exchange trades the authorization code for an identity. For OIDC
	// providers the expectedNonce is verified against the ID token claim;
	// OAuth2-only providers may ignore it.
	Exchange(ctx context.Context, code, expectedNonce string) (*UserIdentity, error)
}

// UserIdentity is the normalized identity returned by any provider.
type UserIdentity struct {
	ProviderName string
	ProviderType string // "basic" | "ldap" | "oidc" | "oauth2"
	ExternalID   string
	Email        string
	Name         string
	AvatarURL    string
	Orgs         []string
	Teams        []string
}

// ProviderInfo describes an external auth provider for the /auth/status endpoint.
type ProviderInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
	Type        string `json:"type"`
}

// User is the persisted user record from SQLite.
type User struct {
	ID           string   `json:"id"`
	ProviderName string   `json:"providerName"`
	ExternalID   string   `json:"externalId"`
	Email        string   `json:"email"`
	Name         string   `json:"name"`
	AvatarURL    string   `json:"avatarUrl"`
	Orgs         []string `json:"orgs"`
	Teams        []string `json:"teams"`
	Roles        []string `json:"roles"`
	LastLogin    string   `json:"lastLogin"`
	CreatedAt    string   `json:"createdAt"`
}
