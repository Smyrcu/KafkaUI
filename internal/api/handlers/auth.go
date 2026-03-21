package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/auth"
	"github.com/Smyrcu/KafkaUI/internal/config"
)

// AuthHandlerDeps groups the dependencies for AuthHandler construction.
type AuthHandlerDeps struct {
	Providers    map[string]auth.IdentityProvider
	ProviderList []auth.ProviderInfo
	Basic        *auth.BasicAuthenticator
	LDAP         *auth.LDAPAuthenticator
	RateLimiter  *auth.LoginRateLimiter
	Sessions     *auth.SessionManager
	UserStore    *auth.UserStore
	RBAC         *auth.RBAC
	AutoRules    []config.AutoAssignmentRule
	DefaultRole  string
	Logger       *slog.Logger
	Enabled      bool
	AuthTypes    []string
	TrustProxy   bool
}

type AuthHandler struct {
	providers    map[string]auth.IdentityProvider
	providerList []auth.ProviderInfo
	basic        *auth.BasicAuthenticator
	ldap         *auth.LDAPAuthenticator
	rateLimiter  *auth.LoginRateLimiter
	sessions     *auth.SessionManager
	userStore    *auth.UserStore
	rbac         *auth.RBAC
	autoRules    []config.AutoAssignmentRule
	defaultRole  string
	logger       *slog.Logger
	enabled      bool
	authTypes    []string
	trustProxy   bool
}

func NewAuthHandler(deps AuthHandlerDeps) *AuthHandler {
	return &AuthHandler{
		providers:    deps.Providers,
		providerList: deps.ProviderList,
		basic:        deps.Basic,
		ldap:         deps.LDAP,
		rateLimiter:  deps.RateLimiter,
		sessions:     deps.Sessions,
		userStore:    deps.UserStore,
		rbac:         deps.RBAC,
		autoRules:    deps.AutoRules,
		defaultRole:  deps.DefaultRole,
		logger:       deps.Logger,
		enabled:      deps.Enabled,
		authTypes:    deps.AuthTypes,
		trustProxy:   deps.TrustProxy,
	}
}

func (h *AuthHandler) hasType(t string) bool {
	return slices.Contains(h.authTypes, t)
}

// hasExternalAuth returns true if "oidc" or "oauth2" is in the configured auth types.
func (h *AuthHandler) hasExternalAuth() bool {
	return h.hasType("oidc") || h.hasType("oauth2")
}

func (h *AuthHandler) LoginBasic(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || (!h.hasType("basic") && !h.hasType("ldap")) {
		writeError(w, http.StatusNotFound, "credential auth not enabled")
		return
	}

	ip := r.RemoteAddr
	if h.trustProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			parts := strings.Split(fwd, ",")
			ip = strings.TrimSpace(parts[0])
		}
	}
	if h.rateLimiter != nil && !h.rateLimiter.Allow(ip) {
		h.logger.Warn("login rate limited", "ip", ip)
		writeError(w, http.StatusTooManyRequests, "too many login attempts, try again later")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password required")
		return
	}

	// Try LDAP first, then fall back to basic auth
	var identity *auth.UserIdentity
	var authErr error

	if h.hasType("ldap") && h.ldap != nil {
		identity, authErr = h.ldap.Authenticate(req.Username, req.Password)
	}
	if (authErr != nil || identity == nil) && h.hasType("basic") && h.basic != nil {
		identity, authErr = h.basic.Authenticate(req.Username, req.Password)
	}
	if authErr != nil || identity == nil {
		h.logger.Warn("login failed", "username", req.Username, "ip", ip)
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Upsert user in the store and resolve roles.
	// For basic auth: if the config declares roles for this user and no admin
	// overrides exist in the DB yet, seed the DB overrides from the config.
	// This makes the config roles field functional as documented.
	user, roles, err := h.upsertAndResolve(identity)
	if err != nil {
		writeInternalError(w, "upserting user", err)
		return
	}
	if h.userStore != nil && h.basic != nil {
		configRoles := h.basic.ConfigRoles(req.Username)
		if len(configRoles) > 0 {
			if dbRoles, err := h.userStore.GetRoles(user.ID); err == nil && len(dbRoles) == 0 {
				for _, role := range configRoles {
					if err := h.userStore.AssignRole(user.ID, role); err != nil {
						h.logger.Warn("failed to seed config role", "user", user.ID, "role", role, "error", err)
					}
				}
				// Re-resolve so the response reflects the newly seeded roles.
				if resolved, err := auth.ResolveRoles(h.userStore, user.ID, identity, h.autoRules, h.defaultRole); err != nil {
					h.logger.Warn("failed to re-resolve roles after seeding", "user", user.ID, "error", err)
				} else {
					roles = resolved
				}
			}
		}
	}

	// Invalidate any existing session before creating a new one
	h.sessions.ClearSession(w, r)

	if err := h.sessions.CreateSession(w, r, auth.SessionData{
		UserID: user.ID,
		Email:  user.Email,
		Name:   user.Name,
	}); err != nil {
		writeInternalError(w, "creating session", err)
		return
	}

	h.logger.Info("login successful", "username", req.Username, "ip", ip)
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"name":          user.Name,
		"roles":         roles,
	})
}

// LoginProvider initiates an external auth flow (OIDC or OAuth2).
func (h *AuthHandler) LoginProvider(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || !h.hasExternalAuth() {
		writeError(w, http.StatusNotFound, "external auth not enabled")
		return
	}

	providerName := chi.URLParam(r, "provider")
	provider, ok := h.providers[providerName]
	if !ok {
		writeError(w, http.StatusNotFound, "unknown provider: "+providerName)
		return
	}

	randomPart, err := generateState()
	if err != nil {
		writeInternalError(w, "generating state", err)
		return
	}
	state := randomPart + ":" + providerName
	nonce, err := generateState()
	if err != nil {
		writeInternalError(w, "generating nonce", err)
		return
	}

	secure := auth.IsSecureRequest(r, h.trustProxy)
	maxAge := int(5 * time.Minute / time.Second)

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_nonce",
		Value:    nonce,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})

	http.Redirect(w, r, provider.AuthCodeURL(state, nonce), http.StatusTemporaryRedirect)
}

func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || !h.hasExternalAuth() {
		writeError(w, http.StatusNotFound, "external auth not enabled")
		return
	}

	if h.rateLimiter != nil {
		ip := r.RemoteAddr
		if h.trustProxy {
			if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
				parts := strings.Split(fwd, ",")
				ip = strings.TrimSpace(parts[0])
			}
		}
		if !h.rateLimiter.Allow(ip) {
			writeError(w, http.StatusTooManyRequests, "too many requests, try again later")
			return
		}
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		writeError(w, http.StatusBadRequest, "missing code or state parameter")
		return
	}

	cookie, err := r.Cookie("oauth_state")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing oauth_state cookie")
		return
	}

	if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(state)) == 0 {
		writeError(w, http.StatusBadRequest, "state mismatch")
		return
	}

	// Extract provider name from state (format: "randomPart:providerName")
	parts := strings.SplitN(state, ":", 2)
	if len(parts) != 2 {
		writeError(w, http.StatusBadRequest, "invalid state format")
		return
	}
	providerName := parts[1]

	provider, ok := h.providers[providerName]
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown provider in state: "+providerName)
		return
	}

	nonceCookie, err := r.Cookie("oauth_nonce")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing oauth_nonce cookie")
		return
	}
	expectedNonce := nonceCookie.Value

	secure := auth.IsSecureRequest(r, h.trustProxy)

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_nonce",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})

	identity, err := provider.Exchange(r.Context(), code, expectedNonce)
	if err != nil {
		writeInternalError(w, "exchanging auth code", err)
		return
	}

	// Upsert user in the store and resolve roles
	user, _, err := h.upsertAndResolve(identity)
	if err != nil {
		writeInternalError(w, "upserting user", err)
		return
	}

	// Invalidate any existing session before creating a new one
	h.sessions.ClearSession(w, r)

	if err := h.sessions.CreateSession(w, r, auth.SessionData{
		UserID: user.ID,
		Email:  user.Email,
		Name:   user.Name,
	}); err != nil {
		writeInternalError(w, "creating session", err)
		return
	}

	redirectURI := "/"
	if redirectCookie, err := r.Cookie("redirect_uri"); err == nil && redirectCookie.Value != "" {
		uri := redirectCookie.Value
		if strings.HasPrefix(uri, "/") && !strings.HasPrefix(uri, "//") {
			redirectURI = uri
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "redirect_uri",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		})
	}

	http.Redirect(w, r, redirectURI, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if session, err := h.sessions.GetSession(r); err == nil {
		h.logger.Info("logout", "name", session.Name)
	}
	h.sessions.ClearSession(w, r)
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		writeJSON(w, http.StatusOK, map[string]any{
			"authenticated": false,
		})
		return
	}

	session, err := h.sessions.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"authenticated": false,
		})
		return
	}

	// Load full user from store to get current roles
	if h.userStore != nil {
		user, err := h.userStore.GetUser(session.UserID)
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"authenticated": true,
				"email":         user.Email,
				"name":          user.Name,
				"roles":         user.Roles,
			})
			return
		}
	}

	// Fallback: return session data without roles
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"email":         session.Email,
		"name":          session.Name,
		"roles":         []string{},
	})
}

func (h *AuthHandler) Status(w http.ResponseWriter, r *http.Request) {
	var types []string
	if h.enabled {
		types = h.authTypes
	}

	var providers []map[string]string
	for _, p := range h.providerList {
		providers = append(providers, map[string]string{
			"name":        p.Name,
			"displayName": p.DisplayName,
			"type":        p.Type,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":   h.enabled,
		"types":     types,
		"providers": providers,
	})
}

func (h *AuthHandler) Permissions(w http.ResponseWriter, r *http.Request) {
	empty := map[string]any{"actions": []string{}, "clusters": []string{}}

	if !h.enabled || h.userStore == nil || h.rbac == nil {
		writeJSON(w, http.StatusOK, empty)
		return
	}

	// Resolve session directly — this endpoint is accessible without the Auth
	// middleware (like /auth/me) so we cannot rely on UserContextKey being set.
	session, err := h.sessions.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusOK, empty)
		return
	}

	user, err := h.userStore.GetUser(session.UserID)
	if err != nil {
		writeJSON(w, http.StatusOK, empty)
		return
	}

	identity := &auth.UserIdentity{Email: user.Email, Orgs: user.Orgs, Teams: user.Teams}
	roles, err := auth.ResolveRoles(h.userStore, session.UserID, identity, h.autoRules, h.defaultRole)
	if err != nil {
		writeJSON(w, http.StatusOK, empty)
		return
	}
	actions := h.rbac.ExpandedActions(roles, "*")

	// TODO: derive per-cluster permissions from RBAC rules for the user's roles.
	// Currently returns ["*"] as a placeholder — the frontend treats this as "all clusters".
	writeJSON(w, http.StatusOK, map[string]any{
		"actions":  actions,
		"clusters": []string{"*"},
	})
}

// upsertAndResolve persists the identity and resolves effective roles.
// The very first user to log in is automatically granted the admin role
// so there is always someone who can access the admin panel.
func (h *AuthHandler) upsertAndResolve(identity *auth.UserIdentity) (*auth.User, []string, error) {
	if h.userStore == nil {
		return &auth.User{
			Email: identity.Email,
			Name:  identity.Name,
		}, []string{}, nil
	}

	user, created, err := h.userStore.UpsertUser(identity)
	if err != nil {
		return nil, nil, err
	}

	// Auto-promote the very first user to admin.
	if created {
		if count, err := h.userStore.UserCount(); err == nil && count == 1 {
			if assignErr := h.userStore.AssignRole(user.ID, "admin"); assignErr != nil {
				h.logger.Warn("failed to auto-assign admin to first user", "user", user.ID, "error", assignErr)
			} else {
				h.logger.Info("first user auto-promoted to admin", "user", user.ID, "email", user.Email)
			}
		}
	}

	roles, err := auth.ResolveRoles(h.userStore, user.ID, identity, h.autoRules, h.defaultRole)
	if err != nil {
		return nil, nil, err
	}
	return user, roles, nil
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand failed: %w", err)
	}
	return hex.EncodeToString(b), nil
}
