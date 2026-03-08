package handlers

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/config"
	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

type HealthHandler struct {
	registry *kafka.Registry
}

func NewHealthHandler(reg *kafka.Registry) *HealthHandler {
	return &HealthHandler{registry: reg}
}

// componentStatus represents the health of a single component.
type componentStatus struct {
	Status   string            `json:"status"`
	Clusters map[string]string `json:"clusters,omitempty"`
}

// healthResponse is the top-level response for readiness checks.
type healthResponse struct {
	Status     string                     `json:"status"`
	Components map[string]componentStatus `json:"components,omitempty"`
}

// Liveness handles GET /healthz — always returns 200 with {"status": "ok"}.
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readiness handles GET /readyz — checks Kafka brokers and optionally external services.
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{
		Status:     "ok",
		Components: make(map[string]componentStatus),
	}

	// Always check Kafka
	kafkaStatus := h.checkKafka(r.Context())
	resp.Components["kafka"] = kafkaStatus
	if kafkaStatus.Status != "ok" {
		resp.Status = "error"
	}

	// Check optional services from ?include= query param
	include := r.URL.Query().Get("include")
	if include != "" {
		services := strings.Split(include, ",")
		for _, svc := range services {
			svc = strings.TrimSpace(svc)
			if svc == "" {
				continue
			}
			switch svc {
			case "schema-registry", "connect", "ksql":
				svcStatus := h.checkExternalService(r.Context(), svc)
				resp.Components[svc] = svcStatus
				if svcStatus.Status == "error" {
					resp.Status = "error"
				}
			}
		}
	}

	status := http.StatusOK
	if resp.Status != "ok" {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, resp)
}

// ServiceCheck handles GET /readyz/{service} — dedicated check for one service.
func (h *HealthHandler) ServiceCheck(w http.ResponseWriter, r *http.Request) {
	service := chi.URLParam(r, "service")

	switch service {
	case "schema-registry", "connect", "ksql":
		svcStatus := h.checkExternalService(r.Context(), service)
		status := http.StatusOK
		if svcStatus.Status == "error" {
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, svcStatus)
	default:
		writeError(w, http.StatusNotFound, "unknown service: "+service)
	}
}

// checkKafka checks all Kafka clusters concurrently with a 3s timeout each.
func (h *HealthHandler) checkKafka(ctx context.Context) componentStatus {
	clusters := h.registry.List()
	result := componentStatus{
		Status:   "ok",
		Clusters: make(map[string]string, len(clusters)),
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, c := range clusters {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			clusterCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()

			client, ok := h.registry.Get(name)
			if !ok {
				mu.Lock()
				result.Clusters[name] = "error"
				result.Status = "error"
				mu.Unlock()
				return
			}

			_, err := client.Brokers(clusterCtx)
			mu.Lock()
			if err != nil {
				result.Clusters[name] = "error"
				result.Status = "error"
			} else {
				result.Clusters[name] = "ok"
			}
			mu.Unlock()
		}(c.Name)
	}

	wg.Wait()
	return result
}

// checkExternalService checks a service across all clusters.
func (h *HealthHandler) checkExternalService(ctx context.Context, service string) componentStatus {
	clusters := h.registry.List()

	// Check if any cluster has this service configured
	anyConfigured := false
	for _, c := range clusters {
		cfg, ok := h.registry.GetConfig(c.Name)
		if !ok {
			continue
		}
		if getServiceURL(cfg, service) != "" {
			anyConfigured = true
			break
		}
	}

	if !anyConfigured {
		return componentStatus{Status: "not_configured"}
	}

	result := componentStatus{
		Status:   "ok",
		Clusters: make(map[string]string),
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, c := range clusters {
		cfg, ok := h.registry.GetConfig(c.Name)
		if !ok {
			continue
		}
		url := getServiceURL(cfg, service)
		if url == "" {
			continue
		}

		wg.Add(1)
		go func(name, checkURL string) {
			defer wg.Done()
			err := httpHealthCheck(ctx, checkURL)
			mu.Lock()
			if err != nil {
				result.Clusters[name] = "error"
				result.Status = "error"
			} else {
				result.Clusters[name] = "ok"
			}
			mu.Unlock()
		}(c.Name, url)
	}

	wg.Wait()
	return result
}

// getServiceURL returns the URL for a given service from cluster config.
func getServiceURL(cfg config.ClusterConfig, service string) string {
	switch service {
	case "schema-registry":
		return cfg.SchemaRegistry.URL
	case "connect":
		if len(cfg.KafkaConnect) > 0 {
			return cfg.KafkaConnect[0].URL
		}
		return ""
	case "ksql":
		return cfg.KSQL.URL
	default:
		return ""
	}
}

// httpHealthCheck performs an HTTP GET to the given URL with a 2s timeout.
func httpHealthCheck(ctx context.Context, url string) error {
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
