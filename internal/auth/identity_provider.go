package auth

import "context"

// IdentityProvider abstracts OIDC and OAuth2 authentication flows.
type IdentityProvider interface {
	Name() string
	Type() string // "oidc" | "oauth2"
	AuthCodeURL(state string) string
	Exchange(ctx context.Context, code string) (*UserIdentity, error)
}

// UserIdentity is the normalized identity returned by any provider.
type UserIdentity struct {
	ProviderName string
	ProviderType string // "oidc" | "oauth2"
	ExternalID   string
	Email        string
	Name         string
	AvatarURL    string
	Orgs         []string
	Teams        []string
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
