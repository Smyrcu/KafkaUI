package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/auth"
	"github.com/Smyrcu/KafkaUI/internal/config"
)

// RBACDeps bundles RBAC middleware dependencies.
type RBACDeps struct {
	RBAC        *auth.RBAC
	Store       *auth.UserStore
	AutoRules   []config.AutoAssignmentRule
	DefaultRole string
}

func RequireAction(deps RBACDeps, action string, authEnabled bool) func(http.Handler) http.Handler {
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

			cluster := chi.URLParam(r, "clusterName")
			if cluster == "" {
				cluster = "*"
			}

			// Use GetUserBasic to avoid a redundant GetRoles query — ResolveRoles
			// will call GetRoles itself when checking for admin overrides.
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

			if !deps.RBAC.IsAllowed(roles, cluster, action) {
				writeJSONError(w, http.StatusForbidden, "forbidden: insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
