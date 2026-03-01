package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/connect"
	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

type ConnectHandler struct {
	registry *kafka.Registry
}

func NewConnectHandler(reg *kafka.Registry) *ConnectHandler {
	return &ConnectHandler{registry: reg}
}

func (h *ConnectHandler) getConnectClients(w http.ResponseWriter, r *http.Request) ([]*connect.Client, bool) {
	clusterName := chi.URLParam(r, "clusterName")
	cfg, ok := h.registry.GetConfig(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return nil, false
	}
	if len(cfg.KafkaConnect) == 0 {
		writeError(w, http.StatusBadRequest, "kafka connect not configured for this cluster")
		return nil, false
	}
	clients := make([]*connect.Client, 0, len(cfg.KafkaConnect))
	for _, kc := range cfg.KafkaConnect {
		clients = append(clients, connect.NewClient(kc.Name, kc.URL))
	}
	return clients, true
}

// findConnector locates a connector by name across all connect clusters.
func (h *ConnectHandler) findConnector(ctx context.Context, clients []*connect.Client, connectorName string) (*connect.Client, *connect.ConnectorDetail, error) {
	for _, client := range clients {
		detail, err := client.GetConnector(ctx, connectorName)
		if err == nil {
			return client, detail, nil
		}
	}
	return nil, nil, fmt.Errorf("connector %q not found in any connect cluster", connectorName)
}

func (h *ConnectHandler) List(w http.ResponseWriter, r *http.Request) {
	clients, ok := h.getConnectClients(w, r)
	if !ok {
		return
	}

	var allConnectors []connect.ConnectorInfo
	for _, client := range clients {
		connectors, err := client.ListConnectors(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		allConnectors = append(allConnectors, connectors...)
	}

	writeJSON(w, http.StatusOK, allConnectors)
}

func (h *ConnectHandler) Details(w http.ResponseWriter, r *http.Request) {
	connectorName := chi.URLParam(r, "connectorName")

	clients, ok := h.getConnectClients(w, r)
	if !ok {
		return
	}

	_, detail, err := h.findConnector(r.Context(), clients, connectorName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (h *ConnectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req connect.CreateConnectorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Config == nil {
		writeError(w, http.StatusBadRequest, "config is required")
		return
	}

	clients, ok := h.getConnectClients(w, r)
	if !ok {
		return
	}

	if len(clients) == 0 {
		writeError(w, http.StatusInternalServerError, "no connect clients available")
		return
	}

	result, err := clients[0].CreateConnector(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (h *ConnectHandler) Update(w http.ResponseWriter, r *http.Request) {
	connectorName := chi.URLParam(r, "connectorName")

	var config map[string]string
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	clients, ok := h.getConnectClients(w, r)
	if !ok {
		return
	}

	client, _, err := h.findConnector(r.Context(), clients, connectorName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	result, err := client.UpdateConnector(r.Context(), connectorName, config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *ConnectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	connectorName := chi.URLParam(r, "connectorName")

	clients, ok := h.getConnectClients(w, r)
	if !ok {
		return
	}

	client, _, err := h.findConnector(r.Context(), clients, connectorName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if err := client.DeleteConnector(r.Context(), connectorName); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *ConnectHandler) Restart(w http.ResponseWriter, r *http.Request) {
	connectorName := chi.URLParam(r, "connectorName")

	clients, ok := h.getConnectClients(w, r)
	if !ok {
		return
	}

	client, _, err := h.findConnector(r.Context(), clients, connectorName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if err := client.RestartConnector(r.Context(), connectorName); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "restarted"})
}

func (h *ConnectHandler) Pause(w http.ResponseWriter, r *http.Request) {
	connectorName := chi.URLParam(r, "connectorName")

	clients, ok := h.getConnectClients(w, r)
	if !ok {
		return
	}

	client, _, err := h.findConnector(r.Context(), clients, connectorName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if err := client.PauseConnector(r.Context(), connectorName); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "paused"})
}

func (h *ConnectHandler) Resume(w http.ResponseWriter, r *http.Request) {
	connectorName := chi.URLParam(r, "connectorName")

	clients, ok := h.getConnectClients(w, r)
	if !ok {
		return
	}

	client, _, err := h.findConnector(r.Context(), clients, connectorName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if err := client.ResumeConnector(r.Context(), connectorName); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "resumed"})
}
