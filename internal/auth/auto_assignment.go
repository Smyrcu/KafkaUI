package auth

import (
	"strings"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

// AutoAssign evaluates rules top-to-bottom against the identity.
// All conditions within a match block use AND logic.
// A user collects roles from ALL matching rules (OR across rules).
// Returns deduplicated role list.
func AutoAssign(rules []config.AutoAssignmentRule, identity *UserIdentity) []string {
	seen := make(map[string]bool)
	var result []string
	for _, rule := range rules {
		if matchesRule(rule.Match, identity) {
			if !seen[rule.Role] {
				seen[rule.Role] = true
				result = append(result, rule.Role)
			}
		}
	}
	return result
}

func matchesRule(match config.AutoAssignmentMatch, identity *UserIdentity) bool {
	conditions := 0
	matched := 0

	if match.Authenticated {
		conditions++
		if identity.Email != "" {
			matched++
		}
	}
	if len(match.Emails) > 0 {
		conditions++
		for _, e := range match.Emails {
			if strings.EqualFold(identity.Email, e) {
				matched++
				break
			}
		}
	}
	if len(match.EmailDomains) > 0 {
		conditions++
		for _, d := range match.EmailDomains {
			if strings.HasSuffix(strings.ToLower(identity.Email), strings.ToLower(d)) {
				matched++
				break
			}
		}
	}
	if len(match.GitHubOrgs) > 0 {
		conditions++
		if hasOverlap(identity.Orgs, match.GitHubOrgs) {
			matched++
		}
	}
	if len(match.GitHubTeams) > 0 {
		conditions++
		if hasOverlap(identity.Teams, match.GitHubTeams) {
			matched++
		}
	}
	if len(match.GitLabGroups) > 0 {
		conditions++
		if hasOverlap(identity.Orgs, match.GitLabGroups) {
			matched++
		}
	}
	return conditions > 0 && matched == conditions
}

func hasOverlap(a, b []string) bool {
	set := make(map[string]bool, len(b))
	for _, v := range b {
		set[v] = true
	}
	for _, v := range a {
		if set[v] {
			return true
		}
	}
	return false
}
