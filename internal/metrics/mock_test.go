package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMockHandler_ReturnsPrometheusFormat(t *testing.T) {
	handler := NewMockHandler()
	req := httptest.NewRequest(http.MethodGet, "/debug/mock-metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("expected text/plain content type, got %s", ct)
	}

	body := rec.Body.String()
	requiredMetrics := []string{
		"kafka_server_brokertopicmetrics_bytesinpersec",
		"kafka_server_brokertopicmetrics_bytesoutpersec",
		"kafka_server_brokertopicmetrics_messagesinpersec",
		"kafka_server_replicamanager_underreplicatedpartitions",
		"kafka_controller_kafkacontroller_activecontrollercount",
		"kafka_controller_kafkacontroller_offlinepartitionscount",
	}
	for _, m := range requiredMetrics {
		if !strings.Contains(body, m) {
			t.Errorf("missing metric %s in response", m)
		}
	}
}

func TestMockHandler_ParseableByOurScraper(t *testing.T) {
	handler := NewMockHandler()
	srv := httptest.NewServer(handler)
	defer srv.Close()

	scraper := NewScraper(srv.URL)
	m, err := scraper.Scrape(t.Context(), "")
	if err != nil {
		t.Fatalf("scraper failed to parse mock metrics: %v", err)
	}
	if m.BytesInPerSec <= 0 {
		t.Error("expected positive BytesInPerSec")
	}
	if m.ActiveControllerCount != 1 {
		t.Errorf("expected ActiveControllerCount=1, got %f", m.ActiveControllerCount)
	}
}
