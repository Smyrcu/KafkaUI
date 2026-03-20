package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"

	"github.com/Smyrcu/KafkaUI/internal/auth"
)

type contextKey string

const UserContextKey contextKey = "user"

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func Auth(sessions *auth.SessionManager, authEnabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !authEnabled {
				next.ServeHTTP(w, r)
				return
			}

			session, err := sessions.GetSession(r)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
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
// Roles are resolved via auth.ResolveRoles (DB roles, auto-assignment, default role).
func RequireRole(role string, authEnabled bool, deps RBACDeps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !authEnabled {
				next.ServeHTTP(w, r)
				return
			}

			session, ok := r.Context().Value(UserContextKey).(*auth.SessionData)
			if !ok || session == nil {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			if deps.Store == nil {
				writeJSONError(w, http.StatusForbidden, "forbidden")
				return
			}

			user, err := deps.Store.GetUserBasic(session.UserID)
			if err != nil {
				writeJSONError(w, http.StatusForbidden, "forbidden")
				return
			}
			identity := &auth.UserIdentity{
				ExternalID: user.ExternalID,
				Email:      user.Email,
				Orgs:       user.Orgs,
				Teams:      user.Teams,
			}
			roles, err := auth.ResolveRoles(deps.Store, session.UserID, identity, deps.AutoRules, deps.DefaultRole)
			if err != nil {
				writeJSONError(w, http.StatusForbidden, "forbidden")
				return
			}

			if !slices.Contains(roles, role) {
				writeJSONError(w, http.StatusForbidden, "forbidden: insufficient role")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
