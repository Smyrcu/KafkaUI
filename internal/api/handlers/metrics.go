package handlers

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
	"github.com/Smyrcu/KafkaUI/internal/metrics"
)

type MetricsHandler struct {
	registry *kafka.Registry
	scrapers map[string]*metrics.Scraper
	store    *metrics.Store
}

func NewMetricsHandler(reg *kafka.Registry, scrapers map[string]*metrics.Scraper, store *metrics.Store) *MetricsHandler {
	return &MetricsHandler{registry: reg, scrapers: scrapers, store: store}
}

type BrokerMetricsResponse struct {
	ID      int32                        `json:"id"`
	Host    string                       `json:"host"`
	Metrics *metrics.BrokerMetrics       `json:"metrics,omitempty"`
	History []metrics.TimestampedMetrics `json:"history,omitempty"`
	Error   string                       `json:"error,omitempty"`
}

type MetricsResponse struct {
	Brokers []BrokerMetricsResponse `json:"brokers"`
}

var rangeDurations = map[string]time.Duration{
	"10m": 10 * time.Minute,
	"1h":  time.Hour,
	"6h":  6 * time.Hour,
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"14d": 14 * 24 * time.Hour,
}

func (h *MetricsHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")

	_, ok := h.scrapers[clusterName]
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

	// Parse time range
	rangeParam := r.URL.Query().Get("range")
	if rangeParam == "" {
		rangeParam = "1h"
	}
	duration, ok := rangeDurations[rangeParam]
	if !ok {
		duration = time.Hour
	}

	// Get latest metrics (live scrape) in parallel
	latest := make([]struct {
		m   *metrics.BrokerMetrics
		err error
	}, len(brokers))
	scraper := h.scrapers[clusterName]
	var wg sync.WaitGroup

	for i, broker := range brokers {
		wg.Add(1)
		go func(idx int, b kafka.BrokerInfo) {
			defer wg.Done()
			m, err := scraper.Scrape(r.Context(), b.Host)
			if err != nil {
				latest[idx].err = err
				return
			}
			latest[idx].m = &m
		}(i, broker)
	}
	wg.Wait()

	results := make([]BrokerMetricsResponse, len(brokers))
	for i, broker := range brokers {
		display := fmt.Sprintf("%s:%d", broker.Host, broker.Port)
		key := fmt.Sprintf("%s:%d", clusterName, broker.ID)

		if latest[i].err != nil {
			results[i] = BrokerMetricsResponse{
				ID:      broker.ID,
				Host:    display,
				History: h.store.Query(key, duration),
				Error:   latest[i].err.Error(),
			}
			continue
		}

		results[i] = BrokerMetricsResponse{
			ID:      broker.ID,
			Host:    display,
			Metrics: latest[i].m,
			History: h.store.Query(key, duration),
		}
	}

	writeJSON(w, http.StatusOK, MetricsResponse{Brokers: results})
}
