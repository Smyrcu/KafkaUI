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
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			if deps.Store == nil {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}

			cluster := chi.URLParam(r, "clusterName")
			if cluster == "" {
				cluster = "*"
			}

			// Use ResolveRoles to get effective roles
			user, err := deps.Store.GetUser(session.UserID)
			if err != nil {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			identity := &auth.UserIdentity{
				Email: user.Email,
				Orgs:  user.Orgs,
				Teams: user.Teams,
			}
			roles, err := auth.ResolveRoles(deps.Store, session.UserID, identity, deps.AutoRules, deps.DefaultRole)
			if err != nil {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}

			if !deps.RBAC.IsAllowed(roles, cluster, action) {
				http.Error(w, `{"error":"forbidden: insufficient permissions"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
