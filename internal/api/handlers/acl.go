package handlers

import (
	"net/http"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

type ACLHandler struct {
	registry *kafka.Registry
}

func NewACLHandler(reg *kafka.Registry) *ACLHandler {
	return &ACLHandler{registry: reg}
}

func (h *ACLHandler) List(w http.ResponseWriter, r *http.Request) {
	client, ok := getClient(h.registry, w, r)
	if !ok {
		return
	}

	acls, err := client.ListACLs(r.Context())
	if err != nil {
		writeInternalError(w, "listing ACLs", err)
		return
	}

	writeJSON(w, http.StatusOK, acls)
}

func (h *ACLHandler) Create(w http.ResponseWriter, r *http.Request) {
	client, ok := getClient(h.registry, w, r)
	if !ok {
		return
	}

	var entry kafka.ACLEntry
	if !decodeBody(w, r, &entry) {
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
		writeInternalError(w, "creating ACL", err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (h *ACLHandler) Delete(w http.ResponseWriter, r *http.Request) {
	client, ok := getClient(h.registry, w, r)
	if !ok {
		return
	}

	var entry kafka.ACLEntry
	if !decodeBody(w, r, &entry) {
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
		writeInternalError(w, "deleting ACL", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
