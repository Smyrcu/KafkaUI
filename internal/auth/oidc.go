package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

// Provider wraps an OIDC identity provider with OAuth2 configuration
// for performing authentication flows and token verification.
type Provider struct {
	oauth2Config oauth2.Config
	oidcVerifier *oidc.IDTokenVerifier
	provider     *oidc.Provider
}

// UserInfo contains the identity claims extracted from an OIDC ID token.
type UserInfo struct {
	Subject string   `json:"sub"`
	Email   string   `json:"email"`
	Name    string   `json:"name"`
	Roles   []string `json:"roles"`
}

// NewProvider creates a new OIDC authentication provider from the given
// configuration. It discovers the issuer's endpoints and configures the
// OAuth2 flow and token verifier.
func NewProvider(ctx context.Context, cfg config.OIDCConfig) (*Provider, error) {
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("creating OIDC provider for issuer %q: %w", cfg.Issuer, err)
	}

	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "profile", "email"}
	}

	oauth2Config := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes:       scopes,
		Endpoint:     provider.Endpoint(),
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: cfg.ClientID,
	})

	return &Provider{
		oauth2Config: oauth2Config,
		oidcVerifier: verifier,
		provider:     provider,
	}, nil
}

// AuthCodeURL returns the URL to redirect the user to for OIDC authorization.
// The state parameter is an opaque value used to prevent CSRF attacks.
func (p *Provider) AuthCodeURL(state string) string {
	return p.oauth2Config.AuthCodeURL(state)
}

// Exchange trades an authorization code for an OAuth2 token, then extracts
// and verifies the embedded OIDC ID token. It returns the parsed user
// information, the raw ID token string, and any error encountered.
func (p *Provider) Exchange(ctx context.Context, code string) (*UserInfo, string, error) {
	token, err := p.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, "", fmt.Errorf("exchanging auth code for token: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, "", fmt.Errorf("no id_token in token response")
	}

	idToken, err := p.oidcVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, "", fmt.Errorf("verifying ID token: %w", err)
	}

	userInfo, err := extractClaims(idToken)
	if err != nil {
		return nil, "", err
	}

	return userInfo, rawIDToken, nil
}

// Verify validates an existing raw ID token string and extracts the user
// information from its claims. This is used to verify tokens on subsequent
// requests after the initial authentication flow.
func (p *Provider) Verify(ctx context.Context, rawToken string) (*UserInfo, error) {
	idToken, err := p.oidcVerifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("verifying ID token: %w", err)
	}

	return extractClaims(idToken)
}

// extractClaims parses the ID token claims into a UserInfo struct. It checks
// multiple common claim keys for roles since different OIDC providers use
// different conventions (e.g., Keycloak uses realm_access.roles, others use
// roles or groups).
func extractClaims(idToken *oidc.IDToken) (*UserInfo, error) {
	var claims struct {
		Subject string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Roles   []string `json:"roles"`
		Groups  []string `json:"groups"`
		RealmAccess struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
	}

	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("extracting claims from ID token: %w", err)
	}

	userInfo := &UserInfo{
		Subject: claims.Subject,
		Email:   claims.Email,
		Name:    claims.Name,
	}

	// Try multiple sources for roles, in priority order.
	switch {
	case len(claims.Roles) > 0:
		userInfo.Roles = claims.Roles
	case len(claims.RealmAccess.Roles) > 0:
		userInfo.Roles = claims.RealmAccess.Roles
	case len(claims.Groups) > 0:
		userInfo.Roles = claims.Groups
	}

	return userInfo, nil
}
