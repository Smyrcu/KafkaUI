package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDynamicConfig_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	dc := NewDynamicConfig(filepath.Join(dir, "clusters.yaml"))

	clusters, err := dc.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clusters != nil {
		t.Fatalf("expected nil, got %v", clusters)
	}
}

func TestDynamicConfig_AddAndLoad(t *testing.T) {
	dir := t.TempDir()
	dc := NewDynamicConfig(filepath.Join(dir, "clusters.yaml"))

	cc := ClusterConfig{
		Name:             "test",
		BootstrapServers: "localhost:9092",
	}
	if err := dc.Add(cc); err != nil {
		t.Fatalf("unexpected error adding cluster: %v", err)
	}

	clusters, err := dc.Load()
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if clusters[0].Name != "test" {
		t.Errorf("expected name 'test', got %q", clusters[0].Name)
	}
	if clusters[0].BootstrapServers != "localhost:9092" {
		t.Errorf("expected bootstrap-servers 'localhost:9092', got %q", clusters[0].BootstrapServers)
	}
}

func TestDynamicConfig_Update(t *testing.T) {
	dir := t.TempDir()
	dc := NewDynamicConfig(filepath.Join(dir, "clusters.yaml"))

	if err := dc.Add(ClusterConfig{Name: "test", BootstrapServers: "localhost:9092"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := ClusterConfig{Name: "test", BootstrapServers: "newhost:9093"}
	if err := dc.Update("test", updated); err != nil {
		t.Fatalf("unexpected error updating: %v", err)
	}

	clusters, err := dc.Load()
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if clusters[0].BootstrapServers != "newhost:9093" {
		t.Errorf("expected 'newhost:9093', got %q", clusters[0].BootstrapServers)
	}
}

func TestDynamicConfig_UpdateNotFound(t *testing.T) {
	dir := t.TempDir()
	dc := NewDynamicConfig(filepath.Join(dir, "clusters.yaml"))

	err := dc.Update("nonexistent", ClusterConfig{Name: "nonexistent", BootstrapServers: "localhost:9092"})
	if err == nil {
		t.Fatal("expected error for updating nonexistent cluster")
	}
}

func TestDynamicConfig_Remove(t *testing.T) {
	dir := t.TempDir()
	dc := NewDynamicConfig(filepath.Join(dir, "clusters.yaml"))

	if err := dc.Add(ClusterConfig{Name: "first", BootstrapServers: "localhost:9092"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := dc.Add(ClusterConfig{Name: "second", BootstrapServers: "localhost:9093"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := dc.Remove("first"); err != nil {
		t.Fatalf("unexpected error removing: %v", err)
	}

	clusters, err := dc.Load()
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if clusters[0].Name != "second" {
		t.Errorf("expected remaining cluster 'second', got %q", clusters[0].Name)
	}
}

func TestDynamicConfig_RemoveNotFound(t *testing.T) {
	dir := t.TempDir()
	dc := NewDynamicConfig(filepath.Join(dir, "clusters.yaml"))

	err := dc.Remove("nonexistent")
	if err == nil {
		t.Fatal("expected error for removing nonexistent cluster")
	}
}

func TestDynamicConfig_AddDuplicate(t *testing.T) {
	dir := t.TempDir()
	dc := NewDynamicConfig(filepath.Join(dir, "clusters.yaml"))

	cc := ClusterConfig{Name: "test", BootstrapServers: "localhost:9092"}
	if err := dc.Add(cc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err := dc.Add(cc)
	if err == nil {
		t.Fatal("expected error for duplicate cluster name")
	}
}

func TestDynamicConfig_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clusters.yaml")
	dc := NewDynamicConfig(path)

	if err := dc.Add(ClusterConfig{Name: "test", BootstrapServers: "localhost:9092"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("expected non-empty file")
	}
}
