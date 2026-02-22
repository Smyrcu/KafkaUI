package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

type ACLHandler struct {
	registry *kafka.Registry
}

func NewACLHandler(reg *kafka.Registry) *ACLHandler {
	return &ACLHandler{registry: reg}
}

func (h *ACLHandler) List(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")
	client, ok := h.registry.Get(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	acls, err := client.ListACLs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, acls)
}

func (h *ACLHandler) Create(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")
	client, ok := h.registry.Get(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	var entry kafka.ACLEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if entry.ResourceType == "" {
		writeError(w, http.StatusBadRequest, "resourceType is required")
		return
	}
	if entry.ResourceName == "" {
		writeError(w, http.StatusBadRequest, "resourceName is required")
		return
	}
	if entry.Principal == "" {
		writeError(w, http.StatusBadRequest, "principal is required")
		return
	}
	if entry.Host == "" {
		writeError(w, http.StatusBadRequest, "host is required (use \"*\" for all hosts)")
		return
	}
	if entry.Operation == "" {
		writeError(w, http.StatusBadRequest, "operation is required")
		return
	}
	if entry.Permission == "" {
		writeError(w, http.StatusBadRequest, "permission is required")
		return
	}
	if entry.PatternType == "" {
		entry.PatternType = "LITERAL"
	}

	if err := client.CreateACL(r.Context(), entry); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (h *ACLHandler) Delete(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")
	client, ok := h.registry.Get(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	var entry kafka.ACLEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if entry.ResourceType == "" {
		writeError(w, http.StatusBadRequest, "resourceType is required")
		return
	}
	if entry.Operation == "" {
		writeError(w, http.StatusBadRequest, "operation is required")
		return
	}

	if err := client.DeleteACL(r.Context(), entry); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
