package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/auth"
)

type AdminUsersHandler struct {
	store *auth.UserStore
}

func NewAdminUsersHandler(store *auth.UserStore) *AdminUsersHandler {
	return &AdminUsersHandler{store: store}
}

func (h *AdminUsersHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListUsers()
	if err != nil {
		writeInternalError(w, "listing users", err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (h *AdminUsersHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.store.GetUser(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *AdminUsersHandler) SetRoles(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Roles []string `json:"roles"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if _, err := h.store.GetUser(id); err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	currentRoles, _ := h.store.GetRoles(id)
	currentSet := make(map[string]bool)
	for _, role := range currentRoles {
		currentSet[role] = true
	}
	newSet := make(map[string]bool)
	for _, role := range req.Roles {
		newSet[role] = true
	}
	for _, role := range currentRoles {
		if !newSet[role] {
			h.store.RemoveRole(id, role)
		}
	}
	for _, role := range req.Roles {
		if !currentSet[role] {
			h.store.AssignRole(id, role)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "roles updated"})
}

func (h *AdminUsersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteUser(id); err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "user deleted"})
}
