package middleware

import (
	"net/http"
	"strings"
)

// RequireJSONContentType rejects state-mutating requests (POST, PUT, DELETE)
// that do not carry a Content-Type containing "application/json".
// This prevents CSRF attacks because browsers cannot send application/json
// from plain HTML forms or cross-origin navigations.
func RequireJSONContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			ct := r.Header.Get("Content-Type")
			if !strings.Contains(ct, "application/json") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnsupportedMediaType)
				w.Write([]byte(`{"error":"Content-Type must be application/json"}`))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
