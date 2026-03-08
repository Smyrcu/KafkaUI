package handlers

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

type DashboardHandler struct {
	registry *kafka.Registry
}

func NewDashboardHandler(reg *kafka.Registry) *DashboardHandler {
	return &DashboardHandler{registry: reg}
}

type ClusterOverview struct {
	Name             string `json:"name"`
	BootstrapServers string `json:"bootstrapServers"`
	BrokerCount      int    `json:"brokerCount"`
	TopicCount       int    `json:"topicCount"`
	ConsumerGroups   int    `json:"consumerGroupCount"`
	Status           string `json:"status"`
}

// Overview returns an overview of all clusters in the registry.
// Each cluster is queried in parallel using goroutines.
func (h *DashboardHandler) Overview(w http.ResponseWriter, r *http.Request) {
	clusters := h.registry.List()

	overviews := make([]ClusterOverview, len(clusters))
	var wg sync.WaitGroup

	for i, c := range clusters {
		wg.Add(1)
		go func(idx int, info kafka.ClusterInfo) {
			defer wg.Done()

			client, ok := h.registry.Get(info.Name)
			if !ok {
				overviews[idx] = ClusterOverview{
					Name:             info.Name,
					BootstrapServers: info.BootstrapServers,
					Status:           "unreachable",
				}
				return
			}

			overviews[idx] = h.getClusterStats(r.Context(), info.Name, client, info.BootstrapServers)
		}(i, c)
	}

	wg.Wait()
	writeJSON(w, http.StatusOK, overviews)
}

// ClusterOverviewDetail returns an overview for a single cluster identified by the clusterName URL param.
func (h *DashboardHandler) ClusterOverviewDetail(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")

	client, ok := getClient(h.registry, w, r)
	if !ok {
		return
	}

	bootstrapServers := clusterName
	if cfg, ok := h.registry.GetConfig(clusterName); ok {
		bootstrapServers = cfg.BootstrapServers
	}
	overview := h.getClusterStats(r.Context(), clusterName, client, bootstrapServers)
	writeJSON(w, http.StatusOK, overview)
}

func (h *DashboardHandler) getClusterStats(ctx context.Context, name string, client *kafka.Client, bootstrapServers string) ClusterOverview {
	overview := ClusterOverview{
		Name:             name,
		BootstrapServers: bootstrapServers,
		Status:           "healthy",
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	brokers, err := client.Brokers(ctx)
	if err != nil {
		overview.Status = "unreachable"
		return overview
	}
	overview.BrokerCount = len(brokers)

	topics, err := client.Topics(ctx)
	if err == nil {
		overview.TopicCount = len(topics)
	} else {
		overview.Status = "degraded"
	}

	groups, err := client.ConsumerGroups(ctx)
	if err == nil {
		overview.ConsumerGroups = len(groups)
	} else {
		if overview.Status != "degraded" {
			overview.Status = "degraded"
		}
	}

	return overview
}
