package middleware

import (
	"context"
	"net/http"

	"github.com/Smyrcu/KafkaUI/internal/auth"
	"github.com/go-chi/chi/v5"
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

// GetUser extracts the session data from the request context.
func GetUser(r *http.Request) *auth.SessionData {
	if data, ok := r.Context().Value(UserContextKey).(*auth.SessionData); ok {
		return data
	}
	return nil
}

func RequireAction(rbac *auth.RBAC, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUser(r)
			if user == nil {
				// Auth not enabled or not authenticated
				next.ServeHTTP(w, r)
				return
			}

			clusterName := chi.URLParam(r, "clusterName")
			if clusterName == "" {
				// Non-cluster routes
				next.ServeHTTP(w, r)
				return
			}

			if !rbac.IsAllowed(user.Roles, clusterName, action) {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
