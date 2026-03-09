package handlers

import (
	"net/http"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

type BrokerHandler struct {
	registry *kafka.Registry
}

func NewBrokerHandler(reg *kafka.Registry) *BrokerHandler {
	return &BrokerHandler{registry: reg}
}

func (h *BrokerHandler) List(w http.ResponseWriter, r *http.Request) {
	client, ok := getClient(h.registry, w, r)
	if !ok {
		return
	}

	brokers, err := client.Brokers(r.Context())
	if err != nil {
		writeInternalError(w, "listing brokers", err)
		return
	}

	writeJSON(w, http.StatusOK, brokers)
}
