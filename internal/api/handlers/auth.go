package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/Smyrcu/KafkaUI/internal/auth"
)

type AuthHandler struct {
	provider *auth.Provider
	sessions *auth.SessionManager
	enabled  bool
}

func NewAuthHandler(provider *auth.Provider, sessions *auth.SessionManager, enabled bool) *AuthHandler {
	return &AuthHandler{provider: provider, sessions: sessions, enabled: enabled}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		writeError(w, http.StatusNotFound, "auth not enabled")
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
	if !h.enabled {
		writeError(w, http.StatusNotFound, "auth not enabled")
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

	// Clear the oauth_state cookie
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

	// Redirect to the original URL if set, otherwise "/"
	redirectURI := "/"
	if redirectCookie, err := r.Cookie("redirect_uri"); err == nil && redirectCookie.Value != "" {
		redirectURI = redirectCookie.Value
		// Clear the redirect_uri cookie
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
	authType := "none"
	if h.enabled {
		authType = "oidc"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": h.enabled,
		"type":    authType,
	})
}

func generateState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
