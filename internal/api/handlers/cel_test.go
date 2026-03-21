package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCELHandler_Validate(t *testing.T) {
	h := NewCELHandler()
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantBody   string
	}{
		{"empty expression", `{"expression":""}`, 400, "expression is required"},
		{"too long", `{"expression":"` + strings.Repeat("x", 1001) + `"}`, 400, "maximum length"},
		{"valid expression", `{"expression":"key == \"test\""}`, 200, `"valid":true`},
		{"invalid syntax", `{"expression":"key =="}`, 200, `"valid":false`},
		{"json field access", `{"expression":"value.status == \"ok\""}`, 200, `"valid":true`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/cel/validate", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.Validate(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want to contain %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}
