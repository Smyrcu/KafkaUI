package handlers

import (
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

