package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMockHandler_ReturnsPrometheusFormat(t *testing.T) {
	handler := MockHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/debug/mock-metrics", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("expected text/plain content-type, got %q", ct)
	}

	body := rec.Body.String()
	expectedMetrics := []string{
		"kafka_server_brokertopicmetrics_bytesinpersec",
		"kafka_server_brokertopicmetrics_bytesoutpersec",
		"kafka_server_brokertopicmetrics_messagesinpersec",
		"kafka_server_replicamanager_underreplicatedpartitions",
		"kafka_controller_kafkacontroller_activecontrollercount",
		"kafka_controller_kafkacontroller_offlinepartitionscount",
		"jvm_memory_used_bytes",
		"jvm_memory_max_bytes",
		"jvm_threads_current",
		"app_http_requests_total",
		"app_http_requests_active",
		"app_request_duration_ms",
		"process_resident_memory_bytes",
		"process_cpu_seconds_total",
		"process_open_fds",
	}

	for _, m := range expectedMetrics {
		if !strings.Contains(body, m) {
			t.Errorf("expected body to contain %q", m)
		}
	}
}

func TestMockHandler_ParseableByOurScraper(t *testing.T) {
	srv := httptest.NewServer(MockHandler())
	defer srv.Close()

	scraper := NewScraper(srv.URL)
	snap, err := scraper.Scrape(context.Background())
	if err != nil {
		t.Fatalf("scraper failed to parse mock output: %v", err)
	}

	if len(snap) < 10 {
		t.Errorf("expected at least 10 metric families, got %d", len(snap))
	}

	// Verify specific metrics are present with expected types
	checks := map[string]string{
		"kafka_server_brokertopicmetrics_bytesinpersec": "gauge",
		"jvm_memory_used_bytes":                         "gauge",
		"app_http_requests_total":                       "gauge",
		"process_open_fds":                              "gauge",
	}

	for name, expectedType := range checks {
		fam, ok := snap[name]
		if !ok {
			t.Errorf("missing metric %q", name)
			continue
		}
		if fam.Type != expectedType {
			t.Errorf("metric %q: expected type %q, got %q", name, expectedType, fam.Type)
		}
		if len(fam.Samples) == 0 {
			t.Errorf("metric %q: expected at least 1 sample", name)
		}
	}

	// Verify multi-sample metric
	jvmMem, ok := snap["jvm_memory_used_bytes"]
	if !ok {
		t.Fatal("missing jvm_memory_used_bytes")
	}
	if len(jvmMem.Samples) < 2 {
		t.Errorf("expected at least 2 samples for jvm_memory_used_bytes (heap + nonheap), got %d", len(jvmMem.Samples))
	}
}
