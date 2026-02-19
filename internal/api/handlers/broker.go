package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

type BrokerHandler struct {
	registry *kafka.Registry
}

func NewBrokerHandler(reg *kafka.Registry) *BrokerHandler {
	return &BrokerHandler{registry: reg}
}

func (h *BrokerHandler) List(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")
	client, ok := h.registry.Get(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	brokers, err := client.Brokers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, brokers)
}
