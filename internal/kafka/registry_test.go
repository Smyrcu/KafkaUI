package kafka

import (
	"testing"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

func TestRegistry_GetByName(t *testing.T) {
	cfg := &config.Config{
		Clusters: []config.ClusterConfig{
			{Name: "cluster-a", BootstrapServers: "localhost:9092"},
			{Name: "cluster-b", BootstrapServers: "localhost:9093"},
		},
	}

	reg, err := NewRegistry(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reg.Close()

	c, ok := reg.Get("cluster-a")
	if !ok {
		t.Fatal("expected to find cluster-a")
	}
	if c.Name() != "cluster-a" {
		t.Errorf("expected name 'cluster-a', got %q", c.Name())
	}

	_, ok = reg.Get("nonexistent")
	if ok {
		t.Fatal("expected not to find nonexistent cluster")
	}
}

func TestRegistry_List(t *testing.T) {
	cfg := &config.Config{
		Clusters: []config.ClusterConfig{
			{Name: "alpha", BootstrapServers: "localhost:9092"},
			{Name: "beta", BootstrapServers: "localhost:9093"},
		},
	}

	reg, err := NewRegistry(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reg.Close()

	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(list))
	}
}

func TestRegistry_EmptyConfig(t *testing.T) {
	cfg := &config.Config{Clusters: []config.ClusterConfig{}}
	reg, err := NewRegistry(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reg.Close()

	list := reg.List()
	if len(list) != 0 {
		t.Fatalf("expected 0 clusters, got %d", len(list))
	}
}

func TestRegistry_GetConfig(t *testing.T) {
	cfg := &config.Config{
		Clusters: []config.ClusterConfig{
			{Name: "test", BootstrapServers: "host1:9092,host2:9092"},
		},
	}
	reg, err := NewRegistry(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reg.Close()

	cc, ok := reg.GetConfig("test")
	if !ok {
		t.Fatal("expected to find config for 'test'")
	}
	if cc.BootstrapServers != "host1:9092,host2:9092" {
		t.Errorf("expected bootstrap servers 'host1:9092,host2:9092', got %q", cc.BootstrapServers)
	}

	_, ok = reg.GetConfig("nonexistent")
	if ok {
		t.Fatal("expected not to find config for 'nonexistent'")
	}
}

func TestRegistry_ListPreservesOrder(t *testing.T) {
	cfg := &config.Config{
		Clusters: []config.ClusterConfig{
			{Name: "charlie", BootstrapServers: "localhost:9092"},
			{Name: "alpha", BootstrapServers: "localhost:9093"},
			{Name: "bravo", BootstrapServers: "localhost:9094"},
		},
	}
	reg, err := NewRegistry(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reg.Close()

	list := reg.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 clusters, got %d", len(list))
	}
	expected := []string{"charlie", "alpha", "bravo"}
	for i, name := range expected {
		if list[i].Name != name {
			t.Errorf("expected cluster[%d] = %q, got %q", i, name, list[i].Name)
		}
	}
}

func TestRegistry_ClientConfig(t *testing.T) {
	cfg := &config.Config{
		Clusters: []config.ClusterConfig{
			{
				Name:             "test",
				BootstrapServers: "kafka:9092",
				SASL: config.SASLConfig{
					Mechanism: "PLAIN",
					Username:  "user",
					Password:  "pass",
				},
			},
		},
	}
	reg, err := NewRegistry(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reg.Close()

	client, ok := reg.Get("test")
	if !ok {
		t.Fatal("expected to find client 'test'")
	}
	if client.Config().SASL.Mechanism != "PLAIN" {
		t.Errorf("expected SASL mechanism 'PLAIN', got %q", client.Config().SASL.Mechanism)
	}
}

func TestRegistry_AddCluster(t *testing.T) {
	cfg := &config.Config{
		Clusters: []config.ClusterConfig{
			{Name: "existing", BootstrapServers: "localhost:9092"},
		},
	}
	reg, err := NewRegistry(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reg.Close()

	cc := config.ClusterConfig{Name: "new-cluster", BootstrapServers: "localhost:9093"}
	if err := reg.AddCluster(cc); err != nil {
		t.Fatalf("unexpected error adding cluster: %v", err)
	}

	// Verify Get works
	client, ok := reg.Get("new-cluster")
	if !ok {
		t.Fatal("expected to find new-cluster via Get")
	}
	if client.Name() != "new-cluster" {
		t.Errorf("expected name 'new-cluster', got %q", client.Name())
	}

	// Verify List includes it
	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(list))
	}
	if list[1].Name != "new-cluster" {
		t.Errorf("expected list[1] = 'new-cluster', got %q", list[1].Name)
	}

	// Verify ClusterCount
	if reg.ClusterCount() != 2 {
		t.Errorf("expected ClusterCount 2, got %d", reg.ClusterCount())
	}
}

func TestRegistry_AddCluster_Duplicate(t *testing.T) {
	cfg := &config.Config{
		Clusters: []config.ClusterConfig{
			{Name: "existing", BootstrapServers: "localhost:9092"},
		},
	}
	reg, err := NewRegistry(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reg.Close()

	cc := config.ClusterConfig{Name: "existing", BootstrapServers: "localhost:9093"}
	if err := reg.AddCluster(cc); err == nil {
		t.Fatal("expected error for duplicate cluster name")
	}
}

func TestRegistry_RemoveCluster(t *testing.T) {
	cfg := &config.Config{
		Clusters: []config.ClusterConfig{
			{Name: "keep", BootstrapServers: "localhost:9092"},
			{Name: "remove-me", BootstrapServers: "localhost:9093"},
		},
	}
	reg, err := NewRegistry(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reg.Close()

	if err := reg.RemoveCluster("remove-me"); err != nil {
		t.Fatalf("unexpected error removing cluster: %v", err)
	}

	// Verify removed from Get
	_, ok := reg.Get("remove-me")
	if ok {
		t.Fatal("expected remove-me to be gone from Get")
	}

	// Verify removed from List
	list := reg.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 cluster in list, got %d", len(list))
	}
	if list[0].Name != "keep" {
		t.Errorf("expected remaining cluster 'keep', got %q", list[0].Name)
	}

	// Verify ClusterCount
	if reg.ClusterCount() != 1 {
		t.Errorf("expected ClusterCount 1, got %d", reg.ClusterCount())
	}
}

func TestRegistry_UpdateCluster(t *testing.T) {
	cfg := &config.Config{
		Clusters: []config.ClusterConfig{
			{Name: "test", BootstrapServers: "localhost:9092"},
		},
	}
	reg, err := NewRegistry(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reg.Close()

	updated := config.ClusterConfig{Name: "test", BootstrapServers: "newhost:9093"}
	if err := reg.UpdateCluster("test", updated); err != nil {
		t.Fatalf("unexpected error updating cluster: %v", err)
	}

	cc, ok := reg.GetConfig("test")
	if !ok {
		t.Fatal("expected to find config for 'test'")
	}
	if cc.BootstrapServers != "newhost:9093" {
		t.Errorf("expected 'newhost:9093', got %q", cc.BootstrapServers)
	}

	// Verify client was replaced (new client works)
	client, ok := reg.Get("test")
	if !ok {
		t.Fatal("expected to find client 'test'")
	}
	if client.Config().BootstrapServers != "newhost:9093" {
		t.Errorf("expected client config 'newhost:9093', got %q", client.Config().BootstrapServers)
	}
}
