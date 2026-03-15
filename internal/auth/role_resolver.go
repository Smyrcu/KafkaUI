package auth

import (
	"github.com/Smyrcu/KafkaUI/internal/config"
)

// ResolveRoles determines a user's effective roles using priority:
// 1. Admin overrides from role_assignments table
// 2. Auto-assignment rules from config
// 3. Default role from config
func ResolveRoles(store *UserStore, userID string, identity *UserIdentity, autoRules []config.AutoAssignmentRule, defaultRole string) []string {
	adminRoles, _ := store.GetRoles(userID)
	if len(adminRoles) > 0 {
		return adminRoles
	}

	autoRoles := AutoAssign(autoRules, identity)
	if len(autoRoles) > 0 {
		return autoRoles
	}

	if defaultRole != "" {
		return []string{defaultRole}
	}
	return nil
}
