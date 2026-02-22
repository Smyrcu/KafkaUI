package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

type ConsumerGroupHandler struct {
	registry *kafka.Registry
}

func NewConsumerGroupHandler(reg *kafka.Registry) *ConsumerGroupHandler {
	return &ConsumerGroupHandler{registry: reg}
}

func (h *ConsumerGroupHandler) List(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")
	client, ok := h.registry.Get(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	groups, err := client.ConsumerGroups(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, groups)
}

func (h *ConsumerGroupHandler) Details(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")
	groupName := chi.URLParam(r, "groupName")

	client, ok := h.registry.Get(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	detail, err := client.ConsumerGroupDetails(r.Context(), groupName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (h *ConsumerGroupHandler) ResetOffsets(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")
	groupName := chi.URLParam(r, "groupName")

	client, ok := h.registry.Get(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	var req kafka.ResetOffsetsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Topic == "" {
		writeError(w, http.StatusBadRequest, "topic is required")
		return
	}
	if req.ResetTo != "earliest" && req.ResetTo != "latest" {
		writeError(w, http.StatusBadRequest, "resetTo must be \"earliest\" or \"latest\"")
		return
	}

	if err := client.ResetConsumerGroupOffsets(r.Context(), groupName, req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "offsets reset", "group": groupName})
}
