package handlers

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/metrics"
)

// MetricsHandler serves generic Prometheus metrics from the store.
type MetricsHandler struct {
	store *metrics.Store
}

// NewMetricsHandler creates a handler that reads metrics from the store.
func NewMetricsHandler(store *metrics.Store) *MetricsHandler {
	return &MetricsHandler{store: store}
}

// MetricGroup is a group of metrics sharing a common prefix.
type MetricGroup struct {
	Name    string         `json:"name"`
	Prefix  string         `json:"prefix"`
	Metrics []MetricDetail `json:"metrics"`
}

// MetricDetail is a single metric with current values and history.
type MetricDetail struct {
	Name    string                `json:"name"`
	Help    string                `json:"help"`
	Type    string                `json:"type"`
	Current []metrics.Sample      `json:"current"`
	History []MetricHistoryPoint  `json:"history"`
}

// MetricHistoryPoint is a single point in a metric's time-series.
type MetricHistoryPoint struct {
	Time  time.Time `json:"time"`
	Value float64   `json:"value"`
}

// MetricsResponse is the API response for the metrics endpoint.
type MetricsResponse struct {
	Groups []MetricGroup `json:"groups"`
}

var rangeDurations = map[string]time.Duration{
	"1m":  time.Minute,
	"5m":  5 * time.Minute,
	"10m": 10 * time.Minute,
	"15m": 15 * time.Minute,
	"30m": 30 * time.Minute,
	"1h":  time.Hour,
	"3h":  3 * time.Hour,
	"6h":  6 * time.Hour,
	"12h": 12 * time.Hour,
	"1d":  24 * time.Hour,
	"24h": 24 * time.Hour,
	"3d":  3 * 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"14d": 14 * 24 * time.Hour,
}

// Metrics handles GET /api/v1/clusters/{clusterName}/metrics.
func (h *MetricsHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")

	if !h.store.HasData(clusterName) {
		writeError(w, http.StatusNotFound, "metrics not configured for this cluster")
		return
	}

	duration := parseDuration(r)

	latest, ok := h.store.GetLatest(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "metrics not configured or no data yet")
		return
	}

	groups := groupMetrics(latest, clusterName, h.store, duration)
	writeJSON(w, http.StatusOK, MetricsResponse{Groups: groups})
}

func parseDuration(r *http.Request) time.Duration {
	fromParam := r.URL.Query().Get("from")
	toParam := r.URL.Query().Get("to")

	if fromParam != "" {
		fromTime, err := time.Parse(time.RFC3339, fromParam)
		if err != nil {
			return time.Hour
		}
		toTime := time.Now()
		if toParam != "" {
			if t, err := time.Parse(time.RFC3339, toParam); err == nil {
				toTime = t
			}
		}
		d := toTime.Sub(fromTime)
		if d <= 0 {
			return time.Hour
		}
		if d > 24*time.Hour {
			return 24 * time.Hour
		}
		return d
	}

	rangeParam := r.URL.Query().Get("range")
	if rangeParam == "" {
		rangeParam = "1h"
	}
	if d, ok := rangeDurations[rangeParam]; ok {
		return d
	}
	return time.Hour
}

func groupMetrics(latest metrics.Snapshot, cluster string, store *metrics.Store, duration time.Duration) []MetricGroup {
	grouped := make(map[string][]MetricDetail)

	for name, fam := range latest {
		prefix := metricPrefix(name)
		history := store.QueryMetric(cluster, name, duration)
		points := make([]MetricHistoryPoint, len(history))
		for i, p := range history {
			points[i] = MetricHistoryPoint{Time: p.Time, Value: p.Value}
		}

		detail := MetricDetail{
			Name:    name,
			Help:    fam.Help,
			Type:    fam.Type,
			Current: fam.Samples,
			History: points,
		}
		grouped[prefix] = append(grouped[prefix], detail)
	}

	groups := make([]MetricGroup, 0, len(grouped))
	for prefix, details := range grouped {
		sort.Slice(details, func(i, j int) bool {
			return details[i].Name < details[j].Name
		})
		groups = append(groups, MetricGroup{
			Name:    groupDisplayName(prefix),
			Prefix:  prefix,
			Metrics: details,
		})
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})
	return groups
}

func metricPrefix(name string) string {
	parts := strings.SplitN(name, "_", 3)
	if len(parts) >= 2 {
		return parts[0] + "_" + parts[1] + "_"
	}
	return name + "_"
}

func groupDisplayName(prefix string) string {
	clean := strings.TrimSuffix(prefix, "_")
	parts := strings.Split(clean, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
