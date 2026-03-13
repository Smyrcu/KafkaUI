package auth

import (
	"github.com/Smyrcu/KafkaUI/internal/config"
)

type RBAC struct {
	rules []config.RBACRule
}

func NewRBAC(rules []config.RBACRule) *RBAC {
	return &RBAC{rules: rules}
}

// IsAllowed checks if any of the user's roles have permission for the given
// action on the given cluster. It returns true as soon as a matching rule is
// found, or false if no rules grant access.
func (r *RBAC) IsAllowed(userRoles []string, cluster string, action string) bool {
	for _, userRole := range userRoles {
		for _, rule := range r.rules {
			if rule.Role != userRole {
				continue
			}
			if !matchesEntry(rule.Clusters, cluster) {
				continue
			}
			if !matchesEntry(rule.Actions, action) {
				continue
			}
			return true
		}
	}
	return false
}

// matchesEntry returns true if the list contains the value or a wildcard "*".
func matchesEntry(list []string, value string) bool {
	for _, entry := range list {
		if entry == "*" || entry == value {
			return true
		}
	}
	return false
}
