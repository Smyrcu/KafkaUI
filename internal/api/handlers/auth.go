package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/Smyrcu/KafkaUI/internal/auth"
)

type AuthHandler struct {
	provider    *auth.Provider
	basic       *auth.BasicAuthenticator
	rateLimiter *auth.LoginRateLimiter
	sessions    *auth.SessionManager
	logger      *slog.Logger
	enabled     bool
	authTypes   []string
}

func NewAuthHandler(provider *auth.Provider, basic *auth.BasicAuthenticator, rateLimiter *auth.LoginRateLimiter, sessions *auth.SessionManager, logger *slog.Logger, enabled bool, authTypes []string) *AuthHandler {
	return &AuthHandler{
		provider:    provider,
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
		ip = fwd
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
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
		writeError(w, http.StatusInternalServerError, "failed to create session")
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

	state := generateState()

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   int(5 * time.Minute / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})

	http.Redirect(w, r, h.provider.AuthCodeURL(state), http.StatusTemporaryRedirect)
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

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	userInfo, rawToken, err := h.provider.Exchange(r.Context(), code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "oauth exchange failed: "+err.Error())
		return
	}

	if err := h.sessions.CreateSession(w, r, auth.SessionData{
		Token: rawToken,
		Email: userInfo.Email,
		Name:  userInfo.Name,
		Roles: userInfo.Roles,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session: "+err.Error())
		return
	}

	redirectURI := "/"
	if redirectCookie, err := r.Cookie("redirect_uri"); err == nil && redirectCookie.Value != "" {
		redirectURI = redirectCookie.Value
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

	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": h.enabled,
		"types":   types,
	})
}

func generateState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
