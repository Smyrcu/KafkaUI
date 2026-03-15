package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/auth"
)

const (
	maxRolesPerUser = 10
	maxRoleLen      = 64
)

var roleNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func validateRole(role string) error {
	if role == "" {
		return fmt.Errorf("role name must not be empty")
	}
	if len(role) > maxRoleLen {
		return fmt.Errorf("role name %q exceeds maximum length of %d", role, maxRoleLen)
	}
	if !roleNameRe.MatchString(role) {
		return fmt.Errorf("role name %q contains invalid characters (allowed: alphanumeric, hyphen, underscore)", role)
	}
	return nil
}

type AdminUsersHandler struct {
	store *auth.UserStore
}

func NewAdminUsersHandler(store *auth.UserStore) *AdminUsersHandler {
	return &AdminUsersHandler{store: store}
}

func (h *AdminUsersHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "user management not available: auth is disabled")
		return
	}
	users, err := h.store.ListUsers()
	if err != nil {
		writeInternalError(w, "listing users", err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (h *AdminUsersHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "user management not available: auth is disabled")
		return
	}
	id := chi.URLParam(r, "id")
	user, err := h.store.GetUser(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *AdminUsersHandler) SetRoles(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "user management not available: auth is disabled")
		return
	}
	id := chi.URLParam(r, "id")
	var req struct {
		Roles []string `json:"roles"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if len(req.Roles) > maxRolesPerUser {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("too many roles: maximum is %d", maxRolesPerUser))
		return
	}
	for _, role := range req.Roles {
		if err := validateRole(role); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if _, err := h.store.GetUser(id); err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	currentRoles, err := h.store.GetRoles(id)
	if err != nil {
		writeInternalError(w, "getting current roles", err)
		return
	}
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
			if err := h.store.RemoveRole(id, role); err != nil {
				writeInternalError(w, "removing role", err)
				return
			}
		}
	}
	for _, role := range req.Roles {
		if !currentSet[role] {
			if err := h.store.AssignRole(id, role); err != nil {
				writeInternalError(w, "assigning role", err)
				return
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "roles updated"})
}

func (h *AdminUsersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "user management not available: auth is disabled")
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteUser(id); err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
		} else {
			writeInternalError(w, "deleting user", err)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "user deleted"})
}
