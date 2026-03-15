package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

// OIDCProvider wraps an OIDC identity provider with OAuth2 configuration
// for performing authentication flows and token verification.
// It implements the IdentityProvider interface.
type OIDCProvider struct {
	name         string
	oauth2Config oauth2.Config
	oidcVerifier *oidc.IDTokenVerifier
	provider     *oidc.Provider
}

// NewOIDCProvider creates a new OIDC authentication provider from the given
// per-provider configuration and shared redirect URL. It discovers the
// issuer's endpoints and configures the OAuth2 flow and token verifier.
func NewOIDCProvider(ctx context.Context, providerCfg config.OIDCProvider, redirectURL string) (*OIDCProvider, error) {
	provider, err := oidc.NewProvider(ctx, providerCfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("creating OIDC provider for issuer %q: %w", providerCfg.Issuer, err)
	}

	scopes := providerCfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "profile", "email"}
	}

	oauth2Config := oauth2.Config{
		ClientID:     providerCfg.ClientID,
		ClientSecret: providerCfg.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       scopes,
		Endpoint:     provider.Endpoint(),
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: providerCfg.ClientID,
	})

	return &OIDCProvider{
		name:         providerCfg.Name,
		oauth2Config: oauth2Config,
		oidcVerifier: verifier,
		provider:     provider,
	}, nil
}

// Name returns the configured name of this provider.
func (p *OIDCProvider) Name() string { return p.name }

// Type returns "oidc".
func (p *OIDCProvider) Type() string { return "oidc" }

// AuthCodeURL returns the URL to redirect the user to for OIDC authorization.
// The state parameter is an opaque value used to prevent CSRF attacks.
func (p *OIDCProvider) AuthCodeURL(state string) string {
	return p.oauth2Config.AuthCodeURL(state)
}

// Exchange trades an authorization code for an OAuth2 token, then extracts
// and verifies the embedded OIDC ID token. It returns the normalized
// UserIdentity and any error encountered.
func (p *OIDCProvider) Exchange(ctx context.Context, code string) (*UserIdentity, error) {
	token, err := p.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchanging auth code for token: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	idToken, err := p.oidcVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verifying ID token: %w", err)
	}

	return extractIdentity(idToken, p.name)
}

// extractIdentity parses the ID token claims into a UserIdentity struct. It
// checks multiple common claim keys for groups/roles since different OIDC
// providers use different conventions (e.g., Keycloak uses realm_access.roles,
// others use groups).
func extractIdentity(idToken *oidc.IDToken, providerName string) (*UserIdentity, error) {
	var claims struct {
		Subject string   `json:"sub"`
		Email   string   `json:"email"`
		Name    string   `json:"name"`
		Picture string   `json:"picture"`
		Roles   []string `json:"roles"`
		Groups  []string `json:"groups"`
		RealmAccess struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
	}

	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("extracting claims from ID token: %w", err)
	}

	identity := &UserIdentity{
		ProviderName: providerName,
		ProviderType: "oidc",
		ExternalID:   claims.Subject,
		Email:        claims.Email,
		Name:         claims.Name,
		AvatarURL:    claims.Picture,
	}

	// Populate Orgs from the most specific group claim available, in priority order.
	switch {
	case len(claims.Groups) > 0:
		identity.Orgs = claims.Groups
	case len(claims.RealmAccess.Roles) > 0:
		identity.Orgs = claims.RealmAccess.Roles
	}

	return identity, nil
}
