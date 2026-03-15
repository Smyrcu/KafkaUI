package middleware

import (
	"context"
	"net/http"
	"slices"

	"github.com/Smyrcu/KafkaUI/internal/auth"
)

type contextKey string

const UserContextKey contextKey = "user"

func Auth(sessions *auth.SessionManager, authEnabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !authEnabled {
				next.ServeHTTP(w, r)
				return
			}

			session, err := sessions.GetSession(r)
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			// Store user info in context
			ctx := context.WithValue(r.Context(), UserContextKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole returns middleware that restricts access to users with the given role.
// When auth is disabled, all requests are allowed through.
// Roles are resolved from the UserStore by the session's UserID.
func RequireRole(role string, authEnabled bool, store *auth.UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !authEnabled {
				next.ServeHTTP(w, r)
				return
			}

			session, ok := r.Context().Value(UserContextKey).(*auth.SessionData)
			if !ok || session == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			if store == nil {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}

			roles, err := store.GetRoles(session.UserID)
			if err != nil {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}

			if !slices.Contains(roles, role) {
				http.Error(w, `{"error":"forbidden: admin role required"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
