package handlers

import (
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
	client, ok := getClient(h.registry, w, r)
	if !ok {
		return
	}

	groups, err := client.ConsumerGroups(r.Context())
	if err != nil {
		writeInternalError(w, "listing consumer groups", err)
		return
	}

	writeJSON(w, http.StatusOK, groups)
}

func (h *ConsumerGroupHandler) Details(w http.ResponseWriter, r *http.Request) {
	groupName := chi.URLParam(r, "groupName")

	client, ok := getClient(h.registry, w, r)
	if !ok {
		return
	}

	detail, err := client.ConsumerGroupDetails(r.Context(), groupName)
	if err != nil {
		writeInternalError(w, "fetching consumer group details", err)
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (h *ConsumerGroupHandler) ResetOffsets(w http.ResponseWriter, r *http.Request) {
	groupName := chi.URLParam(r, "groupName")

	client, ok := getClient(h.registry, w, r)
	if !ok {
		return
	}

	var req kafka.ResetOffsetsRequest
	if !decodeBody(w, r, &req) {
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
		writeInternalError(w, "resetting consumer group offsets", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "offsets reset", "group": groupName})
}
