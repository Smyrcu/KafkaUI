package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/config"
	"github.com/Smyrcu/KafkaUI/internal/kafka"
	"github.com/Smyrcu/KafkaUI/internal/schema"
	"github.com/Smyrcu/KafkaUI/internal/serde"
)

type AdminHandler struct {
	registry    *kafka.Registry
	dynamicCfg  *config.DynamicConfig
	staticNames []string
	serdeChains map[string]kafka.SerDeChain
}

type AdminClusterInfo struct {
	Name             string                      `json:"name"`
	BootstrapServers string                      `json:"bootstrapServers"`
	Dynamic          bool                        `json:"dynamic"`
	TLS              config.TLSConfig            `json:"tls,omitempty"`
	SASL             config.SASLConfig           `json:"sasl,omitempty"`
	SchemaRegistry   config.SchemaRegistryConfig `json:"schemaRegistry,omitempty"`
	KafkaConnect     []config.KafkaConnectConfig `json:"kafkaConnect,omitempty"`
	KSQL             config.KSQLConfig           `json:"ksql,omitempty"`
	Metrics          config.MetricsConfig        `json:"metrics,omitempty"`
}

type AdminClusterList struct {
	Static  []AdminClusterInfo `json:"static"`
	Dynamic []AdminClusterInfo `json:"dynamic"`
}

type AddClusterRequest struct {
	Name             string                      `json:"name"`
	BootstrapServers string                      `json:"bootstrapServers"`
	TLS              config.TLSConfig            `json:"tls"`
	SASL             config.SASLConfig           `json:"sasl"`
	SchemaRegistry   config.SchemaRegistryConfig `json:"schemaRegistry"`
	KafkaConnect     []config.KafkaConnectConfig `json:"kafkaConnect"`
	KSQL             config.KSQLConfig           `json:"ksql"`
	Metrics          config.MetricsConfig        `json:"metrics"`
}

func NewAdminHandler(registry *kafka.Registry, dynamicCfg *config.DynamicConfig, staticNames []string, serdeChains map[string]kafka.SerDeChain) *AdminHandler {
	return &AdminHandler{
		registry:    registry,
		dynamicCfg:  dynamicCfg,
		staticNames: staticNames,
		serdeChains: serdeChains,
	}
}

func (h *AdminHandler) isStatic(name string) bool {
	for _, n := range h.staticNames {
		if n == name {
			return true
		}
	}
	return false
}

func (h *AdminHandler) ListClusters(w http.ResponseWriter, r *http.Request) {
	result := AdminClusterList{
		Static:  []AdminClusterInfo{},
		Dynamic: []AdminClusterInfo{},
	}

	for _, name := range h.staticNames {
		cc, ok := h.registry.GetConfig(name)
		if !ok {
			continue
		}
		result.Static = append(result.Static, AdminClusterInfo{
			Name:             cc.Name,
			BootstrapServers: cc.BootstrapServers,
			Dynamic:          false,
			TLS:              cc.TLS,
			SASL:             cc.SASL,
			SchemaRegistry:   cc.SchemaRegistry,
			KafkaConnect:     cc.KafkaConnect,
			KSQL:             cc.KSQL,
			Metrics:          cc.Metrics,
		})
	}

	dynClusters, err := h.dynamicCfg.Load()
	if err == nil {
		for _, cc := range dynClusters {
			result.Dynamic = append(result.Dynamic, AdminClusterInfo{
				Name:             cc.Name,
				BootstrapServers: cc.BootstrapServers,
				Dynamic:          true,
				TLS:              cc.TLS,
				SASL:             cc.SASL,
				SchemaRegistry:   cc.SchemaRegistry,
				KafkaConnect:     cc.KafkaConnect,
				KSQL:             cc.KSQL,
				Metrics:          cc.Metrics,
			})
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *AdminHandler) AddCluster(w http.ResponseWriter, r *http.Request) {
	var req AddClusterRequest
	if !decodeBody(w, r, &req) {
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.BootstrapServers == "" {
		writeError(w, http.StatusBadRequest, "bootstrapServers is required")
		return
	}

	if _, exists := h.registry.Get(req.Name); exists {
		writeError(w, http.StatusConflict, "cluster already exists")
		return
	}

	cc := requestToClusterConfig(req)

	if r.URL.Query().Get("validate") != "false" {
		if err := testConnection(cc); err != nil {
			writeError(w, http.StatusBadGateway, "connection test failed: unable to reach broker")
			return
		}
	}

	if err := h.dynamicCfg.Add(cc); err != nil {
		writeInternalError(w, "saving dynamic config", err)
		return
	}

	if err := h.registry.AddCluster(cc); err != nil {
		writeInternalError(w, "registering cluster", err)
		return
	}

	// Build SerDe chain for the new cluster
	h.buildSerDeChain(cc)

	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (h *AdminHandler) UpdateCluster(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if h.isStatic(name) {
		writeError(w, http.StatusForbidden, "cannot modify static cluster")
		return
	}

	var req AddClusterRequest
	if !decodeBody(w, r, &req) {
		return
	}

	req.Name = name

	if req.BootstrapServers == "" {
		writeError(w, http.StatusBadRequest, "bootstrapServers is required")
		return
	}

	cc := requestToClusterConfig(req)

	if r.URL.Query().Get("validate") != "false" {
		if err := testConnection(cc); err != nil {
			writeError(w, http.StatusBadGateway, "connection test failed: unable to reach broker")
			return
		}
	}

	if err := h.dynamicCfg.Update(name, cc); err != nil {
		writeInternalError(w, "updating dynamic config", err)
		return
	}

	if err := h.registry.UpdateCluster(name, cc); err != nil {
		writeInternalError(w, "updating cluster", err)
		return
	}

	// Rebuild SerDe chain for the updated cluster
	h.buildSerDeChain(cc)

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *AdminHandler) DeleteCluster(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if h.isStatic(name) {
		writeError(w, http.StatusForbidden, "cannot delete static cluster")
		return
	}

	if h.registry.ClusterCount() <= 1 {
		writeError(w, http.StatusConflict, "cannot delete last cluster")
		return
	}

	if err := h.dynamicCfg.Remove(name); err != nil {
		writeInternalError(w, "removing dynamic config", err)
		return
	}

	if err := h.registry.RemoveCluster(name); err != nil {
		writeInternalError(w, "removing cluster", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *AdminHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	var req AddClusterRequest
	if !decodeBody(w, r, &req) {
		return
	}

	cc := requestToClusterConfig(req)

	if err := testConnection(cc); err != nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "error",
			"error":  "connection failed: unable to reach broker",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func requestToClusterConfig(req AddClusterRequest) config.ClusterConfig {
	return config.ClusterConfig{
		Name:             req.Name,
		BootstrapServers: req.BootstrapServers,
		TLS:              req.TLS,
		SASL:             req.SASL,
		SchemaRegistry:   req.SchemaRegistry,
		KafkaConnect:     req.KafkaConnect,
		KSQL:             req.KSQL,
		Metrics:          req.Metrics,
	}
}

func testConnection(cc config.ClusterConfig) error {
	client, err := kafka.NewClient(cc)
	if err != nil {
		return err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.Brokers(ctx)
	return err
}

// buildSerDeChain creates a SerDe chain for a cluster config and sets it on the kafka client.
func (h *AdminHandler) buildSerDeChain(cc config.ClusterConfig) {
	if h.serdeChains == nil {
		return
	}
	var schemaClient *schema.Client
	if cc.SchemaRegistry.URL != "" {
		schemaClient = schema.NewClient(cc.SchemaRegistry.URL)
	}
	chain := serde.BuildChain(cc.SerDe, schemaClient)
	h.serdeChains[cc.Name] = chain
	if client, ok := h.registry.Get(cc.Name); ok {
		client.SetSerDe(chain)
	}
}
