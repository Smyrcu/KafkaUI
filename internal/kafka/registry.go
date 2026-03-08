package kafka

import (
	"fmt"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

type ClusterInfo struct {
	Name             string `json:"name"`
	BootstrapServers string `json:"bootstrapServers"`
}

type Registry struct {
	clients map[string]*Client
	configs map[string]config.ClusterConfig
	order   []string
}

func NewRegistry(cfg *config.Config) (*Registry, error) {
	r := &Registry{
		clients: make(map[string]*Client, len(cfg.Clusters)),
		configs: make(map[string]config.ClusterConfig, len(cfg.Clusters)),
	}

	for _, cc := range cfg.Clusters {
		client, err := NewClient(cc)
		if err != nil {
			r.Close()
			return nil, fmt.Errorf("creating client for cluster %q: %w", cc.Name, err)
		}
		r.clients[cc.Name] = client
		r.configs[cc.Name] = cc
		r.order = append(r.order, cc.Name)
	}

	return r, nil
}

func (r *Registry) Get(name string) (*Client, bool) {
	c, ok := r.clients[name]
	return c, ok
}

func (r *Registry) GetConfig(name string) (config.ClusterConfig, bool) {
	c, ok := r.configs[name]
	return c, ok
}

func (r *Registry) List() []ClusterInfo {
	result := make([]ClusterInfo, 0, len(r.order))
	for _, name := range r.order {
		cfg := r.configs[name]
		result = append(result, ClusterInfo{
			Name:             cfg.Name,
			BootstrapServers: cfg.BootstrapServers,
		})
	}
	return result
}

func (r *Registry) ClusterCount() int {
	return len(r.order)
}

func (r *Registry) AddCluster(cc config.ClusterConfig) error {
	if _, exists := r.clients[cc.Name]; exists {
		return fmt.Errorf("cluster %q already exists", cc.Name)
	}

	client, err := NewClient(cc)
	if err != nil {
		return fmt.Errorf("creating client for cluster %q: %w", cc.Name, err)
	}

	r.clients[cc.Name] = client
	r.configs[cc.Name] = cc
	r.order = append(r.order, cc.Name)
	return nil
}

func (r *Registry) RemoveCluster(name string) error {
	client, exists := r.clients[name]
	if !exists {
		return fmt.Errorf("cluster %q not found", name)
	}

	client.Close()
	delete(r.clients, name)
	delete(r.configs, name)

	for i, n := range r.order {
		if n == name {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	return nil
}

func (r *Registry) UpdateCluster(name string, cc config.ClusterConfig) error {
	oldClient, exists := r.clients[name]
	if !exists {
		return fmt.Errorf("cluster %q not found", name)
	}

	newClient, err := NewClient(cc)
	if err != nil {
		return fmt.Errorf("creating client for cluster %q: %w", cc.Name, err)
	}

	oldClient.Close()
	r.clients[name] = newClient
	r.configs[name] = cc
	return nil
}

func (r *Registry) Close() {
	for _, c := range r.clients {
		c.Close()
	}
}
