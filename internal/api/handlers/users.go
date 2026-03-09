package handlers

import (
	"net/http"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

type UserHandler struct {
	registry *kafka.Registry
}

func NewUserHandler(reg *kafka.Registry) *UserHandler {
	return &UserHandler{registry: reg}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	client, ok := getClient(h.registry, w, r)
	if !ok {
		return
	}

	users, err := client.ListScramUsers(r.Context())
	if err != nil {
		writeInternalError(w, "listing users", err)
		return
	}

	writeJSON(w, http.StatusOK, users)
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	client, ok := getClient(h.registry, w, r)
	if !ok {
		return
	}

	var req kafka.UpsertScramUserRequest
	if !decodeBody(w, r, &req) {
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}
	if req.Mechanism == "" {
		req.Mechanism = "SCRAM-SHA-256"
	}
	if req.Mechanism != "SCRAM-SHA-256" && req.Mechanism != "SCRAM-SHA-512" {
		writeError(w, http.StatusBadRequest, "mechanism must be SCRAM-SHA-256 or SCRAM-SHA-512")
		return
	}

	if err := client.UpsertScramUser(r.Context(), req); err != nil {
		writeInternalError(w, "creating user", err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	client, ok := getClient(h.registry, w, r)
	if !ok {
		return
	}

	var req struct {
		Name      string `json:"name"`
		Mechanism string `json:"mechanism"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Mechanism == "" {
		writeError(w, http.StatusBadRequest, "mechanism is required")
		return
	}

	if err := client.DeleteScramUser(r.Context(), req.Name, req.Mechanism); err != nil {
		writeInternalError(w, "deleting user", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
