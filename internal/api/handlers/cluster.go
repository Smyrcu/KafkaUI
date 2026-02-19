package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

type ClusterHandler struct {
	registry *kafka.Registry
}

func NewClusterHandler(reg *kafka.Registry) *ClusterHandler {
	return &ClusterHandler{registry: reg}
}

func (h *ClusterHandler) List(w http.ResponseWriter, r *http.Request) {
	clusters := h.registry.List()
	writeJSON(w, http.StatusOK, clusters)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
