package auth

import (
	"testing"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

func TestRBAC_AdminAllowed(t *testing.T) {
	rbac := NewRBAC([]config.RBACRule{
		{Role: "admin", Clusters: []string{"*"}, Actions: []string{"*"}},
	})

	tests := []struct {
		cluster string
		action  string
	}{
		{"production", "view_topics"},
		{"staging", "delete_topics"},
		{"dev", "manage_connectors"},
		{"anything", "any_action"},
	}

	for _, tt := range tests {
		if !rbac.IsAllowed([]string{"admin"}, tt.cluster, tt.action) {
			t.Errorf("expected admin to be allowed for cluster=%q action=%q", tt.cluster, tt.action)
		}
	}
}

func TestRBAC_ViewerLimited(t *testing.T) {
	rbac := NewRBAC([]config.RBACRule{
		{Role: "viewer", Clusters: []string{"production"}, Actions: []string{"view_topics", "view_messages"}},
	})

	// Allowed: production + view_topics
	if !rbac.IsAllowed([]string{"viewer"}, "production", "view_topics") {
		t.Error("expected viewer to be allowed for production/view_topics")
	}

	// Allowed: production + view_messages
	if !rbac.IsAllowed([]string{"viewer"}, "production", "view_messages") {
		t.Error("expected viewer to be allowed for production/view_messages")
	}

	// Denied: production + delete_topics (action not permitted)
	if rbac.IsAllowed([]string{"viewer"}, "production", "delete_topics") {
		t.Error("expected viewer to be denied for production/delete_topics")
	}

	// Denied: staging + view_topics (cluster not permitted)
	if rbac.IsAllowed([]string{"viewer"}, "staging", "view_topics") {
		t.Error("expected viewer to be denied for staging/view_topics")
	}
}

func TestRBAC_MultipleRoles(t *testing.T) {
	rbac := NewRBAC([]config.RBACRule{
		{Role: "viewer", Clusters: []string{"production"}, Actions: []string{"view_topics"}},
		{Role: "operator", Clusters: []string{"staging"}, Actions: []string{"manage_connectors"}},
	})

	roles := []string{"viewer", "operator"}

	// Viewer permission: production/view_topics
	if !rbac.IsAllowed(roles, "production", "view_topics") {
		t.Error("expected viewer role to allow production/view_topics")
	}

	// Operator permission: staging/manage_connectors
	if !rbac.IsAllowed(roles, "staging", "manage_connectors") {
		t.Error("expected operator role to allow staging/manage_connectors")
	}

	// Neither role covers this combination
	if rbac.IsAllowed(roles, "production", "manage_connectors") {
		t.Error("expected denial for production/manage_connectors")
	}

	if rbac.IsAllowed(roles, "staging", "view_topics") {
		t.Error("expected denial for staging/view_topics")
	}
}

func TestRBAC_NoMatchingRole(t *testing.T) {
	rbac := NewRBAC([]config.RBACRule{
		{Role: "admin", Clusters: []string{"*"}, Actions: []string{"*"}},
	})

	if rbac.IsAllowed([]string{"unknown"}, "production", "view_topics") {
		t.Error("expected unknown role to be denied")
	}
}

func TestRBAC_EmptyRoles(t *testing.T) {
	rbac := NewRBAC([]config.RBACRule{
		{Role: "admin", Clusters: []string{"*"}, Actions: []string{"*"}},
	})

	if rbac.IsAllowed([]string{}, "production", "view_topics") {
		t.Error("expected empty roles to be denied")
	}

	if rbac.IsAllowed(nil, "production", "view_topics") {
		t.Error("expected nil roles to be denied")
	}
}

func TestRBAC_WildcardCluster(t *testing.T) {
	rbac := NewRBAC([]config.RBACRule{
		{Role: "monitor", Clusters: []string{"*"}, Actions: []string{"view_topics"}},
	})

	clusters := []string{"production", "staging", "dev", "test", "anything-else"}
	for _, cluster := range clusters {
		if !rbac.IsAllowed([]string{"monitor"}, cluster, "view_topics") {
			t.Errorf("expected wildcard cluster to match %q", cluster)
		}
	}

	// Action still needs to match
	if rbac.IsAllowed([]string{"monitor"}, "production", "delete_topics") {
		t.Error("expected wildcard cluster rule to still enforce action restrictions")
	}
}

func TestRBAC_WildcardAction(t *testing.T) {
	rbac := NewRBAC([]config.RBACRule{
		{Role: "cluster-admin", Clusters: []string{"staging"}, Actions: []string{"*"}},
	})

	actions := []string{"view_topics", "delete_topics", "manage_connectors", "produce_messages", "any_action"}
	for _, action := range actions {
		if !rbac.IsAllowed([]string{"cluster-admin"}, "staging", action) {
			t.Errorf("expected wildcard action to match %q on staging", action)
		}
	}

	// Cluster still needs to match
	if rbac.IsAllowed([]string{"cluster-admin"}, "production", "view_topics") {
		t.Error("expected wildcard action rule to still enforce cluster restrictions")
	}
}

func TestRBAC_GetAllowedClusters(t *testing.T) {
	rbac := NewRBAC([]config.RBACRule{
		{Role: "admin", Clusters: []string{"*"}, Actions: []string{"*"}},
		{Role: "viewer", Clusters: []string{"production"}, Actions: []string{"view_topics"}},
		{Role: "operator", Clusters: []string{"staging", "dev"}, Actions: []string{"manage_connectors"}},
	})

	// Admin gets wildcard
	adminClusters := rbac.GetAllowedClusters([]string{"admin"})
	if len(adminClusters) != 1 || adminClusters[0] != "*" {
		t.Errorf("expected admin to get [\"*\"], got %v", adminClusters)
	}

	// Viewer gets production only
	viewerClusters := rbac.GetAllowedClusters([]string{"viewer"})
	if len(viewerClusters) != 1 || viewerClusters[0] != "production" {
		t.Errorf("expected viewer to get [\"production\"], got %v", viewerClusters)
	}

	// Operator gets staging and dev
	operatorClusters := rbac.GetAllowedClusters([]string{"operator"})
	if len(operatorClusters) != 2 {
		t.Errorf("expected operator to get 2 clusters, got %v", operatorClusters)
	}
	found := map[string]bool{}
	for _, c := range operatorClusters {
		found[c] = true
	}
	if !found["staging"] || !found["dev"] {
		t.Errorf("expected operator clusters to contain staging and dev, got %v", operatorClusters)
	}

	// Multiple roles: viewer + operator gets production, staging, dev
	multiClusters := rbac.GetAllowedClusters([]string{"viewer", "operator"})
	if len(multiClusters) != 3 {
		t.Errorf("expected viewer+operator to get 3 clusters, got %v", multiClusters)
	}
	found = map[string]bool{}
	for _, c := range multiClusters {
		found[c] = true
	}
	if !found["production"] || !found["staging"] || !found["dev"] {
		t.Errorf("expected clusters to contain production, staging, dev, got %v", multiClusters)
	}

	// Empty roles gets nothing
	emptyClusters := rbac.GetAllowedClusters([]string{})
	if len(emptyClusters) != 0 {
		t.Errorf("expected empty roles to get no clusters, got %v", emptyClusters)
	}

	// Unknown role gets nothing
	unknownClusters := rbac.GetAllowedClusters([]string{"unknown"})
	if len(unknownClusters) != 0 {
		t.Errorf("expected unknown role to get no clusters, got %v", unknownClusters)
	}
}
