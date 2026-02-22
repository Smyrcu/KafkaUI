package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
	"github.com/Smyrcu/KafkaUI/internal/ksql"
)

type KsqlHandler struct {
	registry *kafka.Registry
}

func NewKsqlHandler(reg *kafka.Registry) *KsqlHandler {
	return &KsqlHandler{registry: reg}
}

func (h *KsqlHandler) getKsqlClient(w http.ResponseWriter, r *http.Request) (*ksql.Client, bool) {
	clusterName := chi.URLParam(r, "clusterName")
	cfg, ok := h.registry.GetConfig(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return nil, false
	}
	if cfg.KSQL.URL == "" {
		writeError(w, http.StatusBadRequest, "ksql not configured for this cluster")
		return nil, false
	}
	return ksql.NewClient(cfg.KSQL.URL), true
}

func (h *KsqlHandler) Execute(w http.ResponseWriter, r *http.Request) {
	var req ksql.ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	client, ok := h.getKsqlClient(w, r)
	if !ok {
		return
	}

	result, err := client.Execute(r.Context(), req.Query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *KsqlHandler) Info(w http.ResponseWriter, r *http.Request) {
	client, ok := h.getKsqlClient(w, r)
	if !ok {
		return
	}

	info, err := client.Info(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, info)
}
