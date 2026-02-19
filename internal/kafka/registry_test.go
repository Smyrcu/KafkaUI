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
