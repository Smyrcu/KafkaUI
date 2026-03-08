package middleware

import (
	"context"
	"net/http"

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

