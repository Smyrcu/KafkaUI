package auth

import "github.com/Smyrcu/KafkaUI/internal/config"

type RBAC struct {
	expandedRules []expandedRule
	roleGroups    map[string][]string
}

type expandedRule struct {
	role     string
	clusters []string
	actions  []string // fully expanded leaf actions, or ["*"]
}

func NewRBAC(cfg config.RBACConfig) *RBAC {
	r := &RBAC{roleGroups: cfg.RoleGroups}
	for _, rule := range cfg.Rules {
		er := expandedRule{role: rule.Role, clusters: rule.Clusters}
		if containsWildcard(rule.Actions) {
			er.actions = []string{"*"}
		} else {
			er.actions = r.expandActions(rule.Actions)
		}
		r.expandedRules = append(r.expandedRules, er)
	}
	return r
}

// expandActions recursively resolves role group references to leaf actions.
func (r *RBAC) expandActions(actions []string) []string {
	seen := make(map[string]bool)
	var result []string
	var expand func(items []string)
	expand = func(items []string) {
		for _, item := range items {
			if seen[item] {
				continue
			}
			if group, ok := r.roleGroups[item]; ok {
				seen[item] = true // prevent cycles
				expand(group)
			} else {
				seen[item] = true
				result = append(result, item)
			}
		}
	}
	expand(actions)
	return result
}

// IsAllowed checks if any of the user's roles have permission for the given
// action on the given cluster. It returns true as soon as a matching rule is
// found, or false if no rules grant access.
func (r *RBAC) IsAllowed(userRoles []string, cluster string, action string) bool {
	for _, userRole := range userRoles {
		for _, rule := range r.expandedRules {
			if rule.role != userRole {
				continue
			}
			if !matchesEntry(rule.clusters, cluster) {
				continue
			}
			if !matchesEntry(rule.actions, action) {
				continue
			}
			return true
		}
	}
	return false
}

// ExpandedActions returns deduplicated leaf actions for given roles and cluster.
func (r *RBAC) ExpandedActions(userRoles []string, cluster string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, userRole := range userRoles {
		for _, rule := range r.expandedRules {
			if rule.role != userRole {
				continue
			}
			if !matchesEntry(rule.clusters, cluster) {
				continue
			}
			for _, action := range rule.actions {
				if !seen[action] {
					seen[action] = true
					result = append(result, action)
				}
			}
		}
	}
	return result
}

// containsWildcard returns true if the list contains a "*" entry.
func containsWildcard(list []string) bool {
	for _, e := range list {
		if e == "*" {
			return true
		}
	}
	return false
}

// matchesEntry returns true if the list contains the value or a wildcard "*".
func matchesEntry(list []string, value string) bool {
	for _, e := range list {
		if e == "*" || e == value {
			return true
		}
	}
	return false
}
