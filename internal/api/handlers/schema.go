package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
	"github.com/Smyrcu/KafkaUI/internal/schema"
)

type SchemaHandler struct {
	registry *kafka.Registry
}

func NewSchemaHandler(reg *kafka.Registry) *SchemaHandler {
	return &SchemaHandler{registry: reg}
}

func (h *SchemaHandler) getSchemaClient(w http.ResponseWriter, r *http.Request) (*schema.Client, bool) {
	clusterName := chi.URLParam(r, "clusterName")
	cfg, ok := h.registry.GetConfig(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return nil, false
	}
	if cfg.SchemaRegistry.URL == "" {
		writeError(w, http.StatusBadRequest, "schema registry not configured for this cluster")
		return nil, false
	}
	return schema.NewClient(cfg.SchemaRegistry.URL), true
}

func (h *SchemaHandler) List(w http.ResponseWriter, r *http.Request) {
	client, ok := h.getSchemaClient(w, r)
	if !ok {
		return
	}

	subjects, err := client.ListSubjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, subjects)
}

func (h *SchemaHandler) Details(w http.ResponseWriter, r *http.Request) {
	subject := chi.URLParam(r, "subject")

	client, ok := h.getSchemaClient(w, r)
	if !ok {
		return
	}

	details, err := client.GetSubjectDetails(r.Context(), subject)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, details)
}

func (h *SchemaHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req schema.CreateSchemaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Subject == "" {
		writeError(w, http.StatusBadRequest, "subject is required")
		return
	}
	if req.Schema == "" {
		writeError(w, http.StatusBadRequest, "schema is required")
		return
	}

	client, ok := h.getSchemaClient(w, r)
	if !ok {
		return
	}

	result, err := client.CreateSchema(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (h *SchemaHandler) Delete(w http.ResponseWriter, r *http.Request) {
	subject := chi.URLParam(r, "subject")

	client, ok := h.getSchemaClient(w, r)
	if !ok {
		return
	}

	if err := client.DeleteSubject(r.Context(), subject); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "subject": subject})
}
