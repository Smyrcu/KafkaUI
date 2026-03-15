package auth

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const cookieName = "kafkaui_session"

// SessionManager handles cookie-based session management using HMAC-SHA256 signed cookies.
// No external session store is required — the session data is stored directly in the signed cookie.
type SessionManager struct {
	secret []byte
	maxAge int
}

// SessionData holds the user information stored within the session cookie.
//
// Security note: session data is HMAC-SHA256 signed (tamper-proof) but NOT
// encrypted — the JSON payload is base64url-encoded and therefore readable
// by anyone who can access the cookie value. The current fields (UserID,
// Email, Name) are not considered sensitive, but any future additions that
// carry secret information should encrypt the payload before signing.
type SessionData struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
}

// NewSessionManager creates a new SessionManager with the given secret and maximum age in seconds.
// If maxAge is 0, it defaults to 86400 (24 hours).
func NewSessionManager(secret string, maxAge int) *SessionManager {
	if maxAge == 0 {
		maxAge = 86400 // 24 hours
	}
	return &SessionManager{
		secret: []byte(secret),
		maxAge: maxAge,
	}
}

// CreateSession encodes the SessionData as JSON, signs it with HMAC-SHA256, and sets it as
// an HTTP-only cookie. The cookie is Secure when the request arrives over HTTPS (direct TLS
// or X-Forwarded-Proto: https); it is not Secure on localhost so local development works.
func (sm *SessionManager) CreateSession(w http.ResponseWriter, r *http.Request, data SessionData) error {
	data.CreatedAt = time.Now().Unix()
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encoding session data: %w", err)
	}

	signed := sm.sign(jsonData)

	secure := IsSecureRequest(r)

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    string(signed),
		Path:     "/",
		MaxAge:   sm.maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	return nil
}

// GetSession reads the session cookie, verifies the HMAC-SHA256 signature, and returns
// the decoded SessionData. Returns an error if the cookie is missing, the signature is
// invalid, or the data cannot be decoded.
func (sm *SessionManager) GetSession(r *http.Request) (*SessionData, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return nil, fmt.Errorf("session cookie not found: %w", err)
	}

	jsonData, err := sm.verify([]byte(cookie.Value))
	if err != nil {
		return nil, fmt.Errorf("invalid session: %w", err)
	}

	var data SessionData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("decoding session data: %w", err)
	}

	if data.CreatedAt == 0 || time.Now().Unix()-data.CreatedAt > int64(sm.maxAge) {
		return nil, fmt.Errorf("session expired")
	}

	return &data, nil
}

// ClearSession removes the session by setting an expired cookie.
func (sm *SessionManager) ClearSession(w http.ResponseWriter, r *http.Request) {
	secure := IsSecureRequest(r)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
	})
}

// sign produces a signed cookie value in the format: base64url(data).base64url(hmac-sha256(data)).
func (sm *SessionManager) sign(data []byte) []byte {
	mac := hmac.New(sha256.New, sm.secret)
	mac.Write(data)
	sig := mac.Sum(nil)

	encodedData := base64.RawURLEncoding.EncodeToString(data)
	encodedSig := base64.RawURLEncoding.EncodeToString(sig)

	return []byte(encodedData + "." + encodedSig)
}

// verify splits the signed value, decodes the data and signature, and checks the HMAC.
// Returns the original JSON data if the signature is valid.
func (sm *SessionManager) verify(signed []byte) ([]byte, error) {
	parts := bytes.SplitN(signed, []byte("."), 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("malformed session cookie")
	}

	data, err := base64.RawURLEncoding.DecodeString(string(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("decoding session data: %w", err)
	}

	sig, err := base64.RawURLEncoding.DecodeString(string(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("decoding session signature: %w", err)
	}

	mac := hmac.New(sha256.New, sm.secret)
	mac.Write(data)
	expectedSig := mac.Sum(nil)

	if !hmac.Equal(sig, expectedSig) {
		return nil, fmt.Errorf("invalid session signature")
	}

	return data, nil
}

// IsSecureRequest returns true when the request arrives over HTTPS — either
// because the connection itself is TLS, or because a trusted proxy has set
// X-Forwarded-Proto: https. Localhost origins are treated as non-secure so
// that local development works without TLS certificates.
func IsSecureRequest(r *http.Request) bool {
	if isLocalhost(r) {
		return false
	}
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

// isLocalhost returns true if the request originates from a localhost address.
func isLocalhost(r *http.Request) bool {
	host := r.Host
	// Strip port if present.
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
