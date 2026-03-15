package auth

import (
	"fmt"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

// ResolveRoles determines a user's effective roles using priority:
// 1. Admin overrides from role_assignments table
// 2. Auto-assignment rules from config
// 3. Default role from config
//
// An error is returned if the store cannot be queried; callers should deny
// access when an error is returned rather than falling back to a wider role.
func ResolveRoles(store *UserStore, userID string, identity *UserIdentity, autoRules []config.AutoAssignmentRule, defaultRole string) ([]string, error) {
	adminRoles, err := store.GetRoles(userID)
	if err != nil {
		return nil, fmt.Errorf("getting roles for user %s: %w", userID, err)
	}
	if len(adminRoles) > 0 {
		return adminRoles, nil
	}

	autoRoles := AutoAssign(autoRules, identity)
	if len(autoRoles) > 0 {
		return autoRoles, nil
	}

	if defaultRole != "" {
		return []string{defaultRole}, nil
	}
	return nil, nil
}
