package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// dynamicFile is the on-disk YAML structure for dynamic clusters.
type dynamicFile struct {
	Clusters []ClusterConfig `yaml:"clusters"`
}

// DynamicConfig manages dynamic cluster configurations persisted in a YAML file.
type DynamicConfig struct {
	path string
	mu   sync.Mutex
}

// NewDynamicConfig creates a DynamicConfig that reads/writes the given file path.
func NewDynamicConfig(path string) *DynamicConfig {
	return &DynamicConfig{path: path}
}

// Path returns the file path used for persistence.
func (dc *DynamicConfig) Path() string {
	return dc.path
}

// Load reads the dynamic clusters file. Returns nil, nil if the file does not exist.
func (dc *DynamicConfig) Load() ([]ClusterConfig, error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	return dc.loadLocked()
}

func (dc *DynamicConfig) loadLocked() ([]ClusterConfig, error) {
	data, err := os.ReadFile(dc.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading dynamic config: %w", err)
	}

	var f dynamicFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing dynamic config: %w", err)
	}

	return f.Clusters, nil
}

// Add appends a cluster to the dynamic config file. Returns an error if a
// cluster with the same name already exists.
func (dc *DynamicConfig) Add(cluster ClusterConfig) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	clusters, err := dc.loadLocked()
	if err != nil {
		return err
	}

	for _, c := range clusters {
		if c.Name == cluster.Name {
			return fmt.Errorf("cluster %q already exists", cluster.Name)
		}
	}

	clusters = append(clusters, cluster)
	return dc.saveLocked(clusters)
}

// Update replaces the cluster with the given name. Returns an error if no
// cluster with that name exists.
func (dc *DynamicConfig) Update(name string, cluster ClusterConfig) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	clusters, err := dc.loadLocked()
	if err != nil {
		return err
	}

	found := false
	for i, c := range clusters {
		if c.Name == name {
			clusters[i] = cluster
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("cluster %q not found", name)
	}

	return dc.saveLocked(clusters)
}

// Remove deletes the cluster with the given name. Returns an error if no
// cluster with that name exists.
func (dc *DynamicConfig) Remove(name string) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	clusters, err := dc.loadLocked()
	if err != nil {
		return err
	}

	idx := -1
	for i, c := range clusters {
		if c.Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("cluster %q not found", name)
	}

	clusters = append(clusters[:idx], clusters[idx+1:]...)
	return dc.saveLocked(clusters)
}

// saveLocked writes clusters to disk atomically (write temp file, then rename).
// Caller must hold dc.mu.
func (dc *DynamicConfig) saveLocked(clusters []ClusterConfig) error {
	f := dynamicFile{Clusters: clusters}
	data, err := yaml.Marshal(&f)
	if err != nil {
		return fmt.Errorf("marshaling dynamic config: %w", err)
	}

	dir := filepath.Dir(dc.path)
	tmp, err := os.CreateTemp(dir, "dynamic-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpName, dc.path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}
