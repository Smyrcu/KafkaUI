package handlers

import (
	"context"
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
	if len(clients) == 0 {
		return nil, nil, fmt.Errorf("connector %q not found: no connect clusters configured", connectorName)
	}
	var lastErr error
	for _, client := range clients {
		detail, err := client.GetConnector(ctx, connectorName)
		if err == nil {
			return client, detail, nil
		}
		lastErr = err
	}
	return nil, nil, fmt.Errorf("connector %q not found in any connect cluster: %w", connectorName, lastErr)
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
			writeInternalError(w, "listing connectors", err)
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
	if !decodeBody(w, r, &req) {
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
		writeInternalError(w, "no connect clients available", fmt.Errorf("empty connect clients"))
		return
	}

	result, err := clients[0].CreateConnector(r.Context(), req)
	if err != nil {
		writeInternalError(w, "creating connector", err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (h *ConnectHandler) Update(w http.ResponseWriter, r *http.Request) {
	connectorName := chi.URLParam(r, "connectorName")

	var config map[string]string
	if !decodeBody(w, r, &config) {
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
		writeInternalError(w, "updating connector", err)
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
		writeInternalError(w, "deleting connector", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// connectorAction is a shared helper for Restart, Pause, and Resume which all
// follow the same lookup-then-act pattern. actionName is used in the response
// body (e.g. "restarted") and fn performs the actual Connect REST call.
func (h *ConnectHandler) connectorAction(w http.ResponseWriter, r *http.Request, actionName string, fn func(ctx context.Context, client *connect.Client, name string) error) {
	connectorName := chi.URLParam(r, "connectorName")

	clients, ok := h.getConnectClients(w, r)
	if !ok {
		return
	}

	connectClient, _, err := h.findConnector(r.Context(), clients, connectorName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if err := fn(r.Context(), connectClient, connectorName); err != nil {
		writeInternalError(w, "connector action failed", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": actionName})
}

func (h *ConnectHandler) Restart(w http.ResponseWriter, r *http.Request) {
	h.connectorAction(w, r, "restarted", func(ctx context.Context, c *connect.Client, name string) error {
		return c.RestartConnector(ctx, name)
	})
}

func (h *ConnectHandler) Pause(w http.ResponseWriter, r *http.Request) {
	h.connectorAction(w, r, "paused", func(ctx context.Context, c *connect.Client, name string) error {
		return c.PauseConnector(ctx, name)
	})
}

func (h *ConnectHandler) Resume(w http.ResponseWriter, r *http.Request) {
	h.connectorAction(w, r, "resumed", func(ctx context.Context, c *connect.Client, name string) error {
		return c.ResumeConnector(ctx, name)
	})
}
