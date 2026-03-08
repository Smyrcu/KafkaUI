package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
	"github.com/Smyrcu/KafkaUI/internal/testutil"
)

func TestClusterHandler_List(t *testing.T) {
	reg := testutil.MustCreateRegistry(t)
	h := NewClusterHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body []kafka.ClusterInfo
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(body))
	}
	if body[0].Name != "alpha" {
		t.Errorf("expected first cluster 'alpha', got %q", body[0].Name)
	}
}
