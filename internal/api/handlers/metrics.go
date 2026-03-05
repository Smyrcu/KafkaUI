package handlers

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
	"github.com/Smyrcu/KafkaUI/internal/metrics"
)

type MetricsHandler struct {
	registry *kafka.Registry
	scrapers map[string]*metrics.Scraper
}

func NewMetricsHandler(reg *kafka.Registry, scrapers map[string]*metrics.Scraper) *MetricsHandler {
	return &MetricsHandler{registry: reg, scrapers: scrapers}
}

type BrokerMetricsResponse struct {
	ID      int32                  `json:"id"`
	Host    string                 `json:"host"`
	Metrics *metrics.BrokerMetrics `json:"metrics,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

type MetricsResponse struct {
	Brokers []BrokerMetricsResponse `json:"brokers"`
}

func (h *MetricsHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")

	scraper, ok := h.scrapers[clusterName]
	if !ok {
		writeError(w, http.StatusNotFound, "metrics not configured for this cluster")
		return
	}

	if h.registry == nil {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	client, ok := h.registry.Get(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	brokers, err := client.Brokers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("listing brokers: %v", err))
		return
	}

	results := make([]BrokerMetricsResponse, len(brokers))
	var wg sync.WaitGroup

	for i, broker := range brokers {
		wg.Add(1)
		go func(idx int, b kafka.BrokerInfo) {
			defer wg.Done()
			display := fmt.Sprintf("%s:%d", b.Host, b.Port)
			m, err := scraper.Scrape(r.Context(), b.Host)
			if err != nil {
				results[idx] = BrokerMetricsResponse{
					ID:    b.ID,
					Host:  display,
					Error: err.Error(),
				}
				return
			}
			results[idx] = BrokerMetricsResponse{
				ID:      b.ID,
				Host:    display,
				Metrics: &m,
			}
		}(i, broker)
	}

	wg.Wait()
	writeJSON(w, http.StatusOK, MetricsResponse{Brokers: results})
}
