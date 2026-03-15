package handlers

import (
	"crypto/rand"
	"encoding/hex"
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
	RateLimiter  *auth.LoginRateLimiter
	Sessions     *auth.SessionManager
	UserStore    *auth.UserStore
	RBAC         *auth.RBAC
	AutoRules    []config.AutoAssignmentRule
	DefaultRole  string
	Logger       *slog.Logger
	Enabled      bool
	AuthTypes    []string
}

type AuthHandler struct {
	providers    map[string]auth.IdentityProvider
	providerList []auth.ProviderInfo
	basic        *auth.BasicAuthenticator
	rateLimiter  *auth.LoginRateLimiter
	sessions     *auth.SessionManager
	userStore    *auth.UserStore
	rbac         *auth.RBAC
	autoRules    []config.AutoAssignmentRule
	defaultRole  string
	logger       *slog.Logger
	enabled      bool
	authTypes    []string
}

func NewAuthHandler(deps AuthHandlerDeps) *AuthHandler {
	return &AuthHandler{
		providers:    deps.Providers,
		providerList: deps.ProviderList,
		basic:        deps.Basic,
		rateLimiter:  deps.RateLimiter,
		sessions:     deps.Sessions,
		userStore:    deps.UserStore,
		rbac:         deps.RBAC,
		autoRules:    deps.AutoRules,
		defaultRole:  deps.DefaultRole,
		logger:       deps.Logger,
		enabled:      deps.Enabled,
		authTypes:    deps.AuthTypes,
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
	if !h.enabled || !h.hasType("basic") {
		writeError(w, http.StatusNotFound, "basic auth not enabled")
		return
	}

	ip := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		ip = strings.TrimSpace(parts[len(parts)-1])
	}
	if !h.rateLimiter.Allow(ip) {
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

	identity, err := h.basic.Authenticate(req.Username, req.Password)
	if err != nil {
		h.logger.Warn("login failed", "username", req.Username, "ip", ip)
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Upsert user in the store and resolve roles
	user, roles, err := h.upsertAndResolve(identity)
	if err != nil {
		writeInternalError(w, "upserting user", err)
		return
	}

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

	randomPart := generateState()
	state := randomPart + ":" + providerName

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   int(5 * time.Minute / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	})

	http.Redirect(w, r, provider.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || !h.hasExternalAuth() {
		writeError(w, http.StatusNotFound, "external auth not enabled")
		return
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

	if cookie.Value != state {
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

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	identity, err := provider.Exchange(r.Context(), code)
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

// upsertAndResolve persists the identity and resolves effective roles.
func (h *AuthHandler) upsertAndResolve(identity *auth.UserIdentity) (*auth.User, []string, error) {
	if h.userStore == nil {
		return &auth.User{
			Email: identity.Email,
			Name:  identity.Name,
		}, []string{}, nil
	}

	user, _, err := h.userStore.UpsertUser(identity)
	if err != nil {
		return nil, nil, err
	}

	roles := auth.ResolveRoles(h.userStore, user.ID, identity, h.autoRules, h.defaultRole)
	return user, roles, nil
}

func generateState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
