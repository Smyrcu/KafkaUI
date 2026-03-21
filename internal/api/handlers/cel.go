package handlers

import (
	"net/http"

	celfilter "github.com/Smyrcu/KafkaUI/internal/cel"
)

const maxCELExpressionLen = 1000

// CELHandler handles CEL expression operations.
type CELHandler struct{}

func NewCELHandler() *CELHandler { return &CELHandler{} }

func (h *CELHandler) Validate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Expression string `json:"expression"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Expression == "" {
		writeError(w, http.StatusBadRequest, "expression is required")
		return
	}
	if len(req.Expression) > maxCELExpressionLen {
		writeError(w, http.StatusBadRequest, "expression exceeds maximum length of 1000 bytes")
		return
	}
	if _, err := celfilter.NewFilter(req.Expression); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true})
}
