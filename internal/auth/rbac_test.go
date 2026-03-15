package auth

import (
	"sort"
	"testing"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

// roleGroups used across multiple tests:
//
//	view  = [view_topics, view_messages, view_consumer_groups]
//	edit  = [view, create_topics, produce_messages]
//	admin = [edit, delete_topics, manage_connectors]
func buildGroupedRBAC(rules []config.RBACRule) *RBAC {
	return NewRBAC(config.RBACConfig{
		RoleGroups: map[string][]string{
			"view":  {"view_topics", "view_messages", "view_consumer_groups"},
			"edit":  {"view", "create_topics", "produce_messages"},
			"admin": {"edit", "delete_topics", "manage_connectors"},
		},
		Rules: rules,
	})
}

// TestRBAC_RoleGroupExpansion verifies that role group references are
// recursively resolved at construction time.
func TestRBAC_RoleGroupExpansion(t *testing.T) {
	rbac := buildGroupedRBAC([]config.RBACRule{
		{Role: "viewer", Clusters: []string{"*"}, Actions: []string{"view"}},
		{Role: "editor", Clusters: []string{"*"}, Actions: []string{"edit"}},
		{Role: "admin", Clusters: []string{"*"}, Actions: []string{"*"}},
	})

	// viewer: inherits view group → can view_topics, cannot create_topics
	if !rbac.IsAllowed([]string{"viewer"}, "production", "view_topics") {
		t.Error("viewer should be allowed view_topics")
	}
	if !rbac.IsAllowed([]string{"viewer"}, "production", "view_messages") {
		t.Error("viewer should be allowed view_messages")
	}
	if rbac.IsAllowed([]string{"viewer"}, "production", "create_topics") {
		t.Error("viewer should NOT be allowed create_topics")
	}

	// editor: inherits edit → view → can create_topics AND view_topics
	if !rbac.IsAllowed([]string{"editor"}, "staging", "create_topics") {
		t.Error("editor should be allowed create_topics via edit group")
	}
	if !rbac.IsAllowed([]string{"editor"}, "staging", "view_topics") {
		t.Error("editor should be allowed view_topics via edit->view chain")
	}
	if rbac.IsAllowed([]string{"editor"}, "staging", "delete_topics") {
		t.Error("editor should NOT be allowed delete_topics")
	}

	// admin: wildcard → any action allowed
	if !rbac.IsAllowed([]string{"admin"}, "production", "delete_topics") {
		t.Error("admin should be allowed delete_topics via wildcard")
	}
	if !rbac.IsAllowed([]string{"admin"}, "production", "anything_at_all") {
		t.Error("admin should be allowed any action via wildcard")
	}
}

// TestRBAC_MixedGroupAndLiteralActions verifies that a rule can mix a role
// group reference with a literal action name.
func TestRBAC_MixedGroupAndLiteralActions(t *testing.T) {
	rbac := buildGroupedRBAC([]config.RBACRule{
		{Role: "operator", Clusters: []string{"*"}, Actions: []string{"view", "produce_messages"}},
	})

	// Expanded from "view" group
	if !rbac.IsAllowed([]string{"operator"}, "production", "view_topics") {
		t.Error("operator should be allowed view_topics via view group")
	}
	if !rbac.IsAllowed([]string{"operator"}, "production", "view_messages") {
		t.Error("operator should be allowed view_messages via view group")
	}

	// Literal action
	if !rbac.IsAllowed([]string{"operator"}, "production", "produce_messages") {
		t.Error("operator should be allowed produce_messages (literal)")
	}

	// Not in view group or literal list
	if rbac.IsAllowed([]string{"operator"}, "production", "delete_topics") {
		t.Error("operator should NOT be allowed delete_topics")
	}
}

// TestRBAC_ClusterScoped verifies that a viewer on one cluster is denied on
// another cluster even when the action matches.
func TestRBAC_ClusterScoped(t *testing.T) {
	rbac := buildGroupedRBAC([]config.RBACRule{
		{Role: "viewer", Clusters: []string{"staging"}, Actions: []string{"view"}},
	})

	if !rbac.IsAllowed([]string{"viewer"}, "staging", "view_topics") {
		t.Error("viewer should be allowed on staging")
	}
	if rbac.IsAllowed([]string{"viewer"}, "production", "view_topics") {
		t.Error("viewer should be DENIED on production (cluster-scoped)")
	}
}

// TestRBAC_EmptyRoles verifies that nil and empty role slices are always denied.
func TestRBAC_EmptyRoles(t *testing.T) {
	rbac := buildGroupedRBAC([]config.RBACRule{
		{Role: "admin", Clusters: []string{"*"}, Actions: []string{"*"}},
	})

	if rbac.IsAllowed([]string{}, "production", "view_topics") {
		t.Error("empty roles should be denied")
	}
	if rbac.IsAllowed(nil, "production", "view_topics") {
		t.Error("nil roles should be denied")
	}
}

// TestRBAC_NoRoleGroups verifies backwards-compatible behaviour: when no role
// groups are defined, literal action names work exactly as before.
func TestRBAC_NoRoleGroups(t *testing.T) {
	rbac := NewRBAC(config.RBACConfig{
		Rules: []config.RBACRule{
			{Role: "viewer", Clusters: []string{"production"}, Actions: []string{"view_topics", "view_messages"}},
			{Role: "admin", Clusters: []string{"*"}, Actions: []string{"*"}},
		},
	})

	if !rbac.IsAllowed([]string{"viewer"}, "production", "view_topics") {
		t.Error("viewer should be allowed view_topics on production")
	}
	if rbac.IsAllowed([]string{"viewer"}, "staging", "view_topics") {
		t.Error("viewer should be denied on staging (no role groups)")
	}
	if rbac.IsAllowed([]string{"viewer"}, "production", "delete_topics") {
		t.Error("viewer should be denied delete_topics")
	}
	if !rbac.IsAllowed([]string{"admin"}, "anything", "anything") {
		t.Error("admin wildcard should allow anything")
	}
}

// TestRBAC_ExpandedActions verifies that ExpandedActions returns a correct,
// deduplicated list of leaf actions for the given roles and cluster.
func TestRBAC_ExpandedActions(t *testing.T) {
	rbac := buildGroupedRBAC([]config.RBACRule{
		{Role: "viewer", Clusters: []string{"*"}, Actions: []string{"view"}},
		{Role: "editor", Clusters: []string{"*"}, Actions: []string{"edit"}},
	})

	// viewer: view group expands to 3 leaf actions
	viewerActions := rbac.ExpandedActions([]string{"viewer"}, "production")
	sort.Strings(viewerActions)
	wantViewer := []string{"view_consumer_groups", "view_messages", "view_topics"}
	if !equalStringSlices(viewerActions, wantViewer) {
		t.Errorf("viewer expanded actions = %v, want %v", viewerActions, wantViewer)
	}

	// editor: edit group expands to view (3) + create_topics + produce_messages = 5 leaf actions
	editorActions := rbac.ExpandedActions([]string{"editor"}, "production")
	sort.Strings(editorActions)
	wantEditor := []string{"create_topics", "produce_messages", "view_consumer_groups", "view_messages", "view_topics"}
	if !equalStringSlices(editorActions, wantEditor) {
		t.Errorf("editor expanded actions = %v, want %v", editorActions, wantEditor)
	}

	// viewer+editor combined: should be deduplicated (same 5 as editor alone)
	combined := rbac.ExpandedActions([]string{"viewer", "editor"}, "production")
	sort.Strings(combined)
	if !equalStringSlices(combined, wantEditor) {
		t.Errorf("viewer+editor combined actions = %v, want %v", combined, wantEditor)
	}

	// cluster mismatch: no actions
	noActions := rbac.ExpandedActions([]string{"viewer"}, "production")
	if len(noActions) == 0 {
		// sanity: production should have actions (tested above)
	}
	missActions := rbac.ExpandedActions([]string{"viewer"}, "production")
	_ = missActions // already tested

	// rules scoped to staging should yield nothing for production when scoped
	rbac2 := buildGroupedRBAC([]config.RBACRule{
		{Role: "viewer", Clusters: []string{"staging"}, Actions: []string{"view"}},
	})
	empty := rbac2.ExpandedActions([]string{"viewer"}, "production")
	if len(empty) != 0 {
		t.Errorf("expected no actions for viewer on production, got %v", empty)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
