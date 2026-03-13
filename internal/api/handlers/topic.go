package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

type TopicHandler struct {
	registry *kafka.Registry
}

func NewTopicHandler(reg *kafka.Registry) *TopicHandler {
	return &TopicHandler{registry: reg}
}

func (h *TopicHandler) List(w http.ResponseWriter, r *http.Request) {
	client, ok := getClient(h.registry, w, r)
	if !ok {
		return
	}

	topics, err := client.Topics(r.Context())
	if err != nil {
		writeInternalError(w, "listing topics", err)
		return
	}

	writeJSON(w, http.StatusOK, topics)
}

func (h *TopicHandler) Details(w http.ResponseWriter, r *http.Request) {
	topicName := chi.URLParam(r, "topicName")

	client, ok := getClient(h.registry, w, r)
	if !ok {
		return
	}

	detail, err := client.TopicDetails(r.Context(), topicName)
	if err != nil {
		writeInternalError(w, "fetching topic details", err)
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (h *TopicHandler) Create(w http.ResponseWriter, r *http.Request) {
	client, ok := getClient(h.registry, w, r)
	if !ok {
		return
	}

	var req kafka.CreateTopicRequest
	if !decodeBody(w, r, &req) {
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "topic name is required")
		return
	}
	if req.Partitions <= 0 {
		req.Partitions = 1
	}
	if req.Replicas <= 0 {
		req.Replicas = 1
	}

	if err := client.CreateTopic(r.Context(), req); err != nil {
		writeInternalError(w, "creating topic", err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "created", "topic": req.Name})
}

func (h *TopicHandler) Delete(w http.ResponseWriter, r *http.Request) {
	topicName := chi.URLParam(r, "topicName")

	client, ok := getClient(h.registry, w, r)
	if !ok {
		return
	}

	if err := client.DeleteTopic(r.Context(), topicName); err != nil {
		writeInternalError(w, "deleting topic", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "topic": topicName})
}
