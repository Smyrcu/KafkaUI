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

type AuthHandler struct {
	providers   map[string]*auth.Provider
	providerCfg []config.OIDCProvider
	basic       *auth.BasicAuthenticator
	rateLimiter *auth.LoginRateLimiter
	sessions    *auth.SessionManager
	logger      *slog.Logger
	enabled     bool
	authTypes   []string
}

func NewAuthHandler(providers map[string]*auth.Provider, providerCfg []config.OIDCProvider, basic *auth.BasicAuthenticator, rateLimiter *auth.LoginRateLimiter, sessions *auth.SessionManager, logger *slog.Logger, enabled bool, authTypes []string) *AuthHandler {
	return &AuthHandler{
		providers:   providers,
		providerCfg: providerCfg,
		basic:       basic,
		rateLimiter: rateLimiter,
		sessions:    sessions,
		logger:      logger,
		enabled:     enabled,
		authTypes:   authTypes,
	}
}

func (h *AuthHandler) hasType(t string) bool {
	return slices.Contains(h.authTypes, t)
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

	session, err := h.basic.Authenticate(req.Username, req.Password)
	if err != nil {
		h.logger.Warn("login failed", "username", req.Username, "ip", ip)
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := h.sessions.CreateSession(w, r, *session); err != nil {
		writeInternalError(w, "creating session", err)
		return
	}

	h.logger.Info("login successful", "username", req.Username, "ip", ip)
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"name":          session.Name,
		"roles":         session.Roles,
	})
}

func (h *AuthHandler) LoginOIDC(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || !h.hasType("oidc") {
		writeError(w, http.StatusNotFound, "oidc auth not enabled")
		return
	}

	providerName := chi.URLParam(r, "provider")
	provider, ok := h.providers[providerName]
	if !ok {
		writeError(w, http.StatusNotFound, "unknown OIDC provider: "+providerName)
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
	if !h.enabled || !h.hasType("oidc") {
		writeError(w, http.StatusNotFound, "oidc auth not enabled")
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
		writeError(w, http.StatusBadRequest, "unknown OIDC provider in state: "+providerName)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	userInfo, rawToken, err := provider.Exchange(r.Context(), code)
	if err != nil {
		writeInternalError(w, "exchanging OIDC code", err)
		return
	}

	if err := h.sessions.CreateSession(w, r, auth.SessionData{
		Token: rawToken,
		Email: userInfo.Email,
		Name:  userInfo.Name,
		Roles: userInfo.Roles,
	}); err != nil {
		writeInternalError(w, "creating OIDC session", err)
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

	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"email":         session.Email,
		"name":          session.Name,
		"roles":         session.Roles,
	})
}

func (h *AuthHandler) Status(w http.ResponseWriter, r *http.Request) {
	var types []string
	if h.enabled {
		types = h.authTypes
	}

	var providers []map[string]string
	for _, p := range h.providerCfg {
		providers = append(providers, map[string]string{
			"name":        p.Name,
			"displayName": p.DisplayName,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":   h.enabled,
		"types":     types,
		"providers": providers,
	})
}

func generateState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
