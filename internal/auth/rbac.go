package auth

import (
	"github.com/Smyrcu/KafkaUI/internal/config"
)

const (
	ActionViewTopics         = "view_topics"
	ActionCreateTopics       = "create_topics"
	ActionDeleteTopics       = "delete_topics"
	ActionViewMessages       = "view_messages"
	ActionProduceMessages    = "produce_messages"
	ActionViewConsumerGroups = "view_consumer_groups"
	ActionResetOffsets       = "reset_offsets"
	ActionViewSchemas        = "view_schemas"
	ActionManageSchemas      = "manage_schemas"
	ActionViewConnectors     = "view_connectors"
	ActionManageConnectors   = "manage_connectors"
	ActionExecuteKSQL        = "execute_ksql"
	ActionViewACLs           = "view_acls"
	ActionManageACLs         = "manage_acls"
	ActionViewUsers          = "view_users"
	ActionManageUsers        = "manage_users"
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
