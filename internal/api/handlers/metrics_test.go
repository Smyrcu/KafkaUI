package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/metrics"
)

func TestMetricsHandler_NotConfigured(t *testing.T) {
	h := NewMetricsHandler(metrics.NewStore())

	r := chi.NewRouter()
	r.Get("/clusters/{clusterName}/metrics", h.Metrics)

	req := httptest.NewRequest(http.MethodGet, "/clusters/test/metrics", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestMetricsHandler_WithData(t *testing.T) {
	store := metrics.NewStore()
	store.Append("test-cluster", metrics.Snapshot{
		"kafka_server_bytesinpersec": metrics.MetricFamily{
			Name: "kafka_server_bytesinpersec",
			Help: "Bytes in per second",
			Type: "gauge",
			Samples: []metrics.Sample{
				{Value: 1234.5, Labels: map[string]string{"topic": ""}},
			},
		},
		"jvm_memory_used_bytes": metrics.MetricFamily{
			Name: "jvm_memory_used_bytes",
			Help: "JVM memory used",
			Type: "gauge",
			Samples: []metrics.Sample{
				{Value: 500000000, Labels: map[string]string{"area": "heap"}},
			},
		},
	})

	h := NewMetricsHandler(store)

	r := chi.NewRouter()
	r.Get("/clusters/{clusterName}/metrics", h.Metrics)

	req := httptest.NewRequest(http.MethodGet, "/clusters/test-cluster/metrics?range=1h", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp MetricsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(resp.Groups))
	}

	// Groups should be sorted alphabetically
	if resp.Groups[0].Name != "Jvm Memory" {
		t.Errorf("expected first group 'Jvm Memory', got %q", resp.Groups[0].Name)
	}
	if resp.Groups[1].Name != "Kafka Server" {
		t.Errorf("expected second group 'Kafka Server', got %q", resp.Groups[1].Name)
	}
}

func TestMetricPrefix(t *testing.T) {
	tests := []struct {
		name   string
		want   string
	}{
		{"debezium_streaming_connected", "debezium_streaming_"},
		{"kafka_connect_sink_task_put_batch_avg_time_ms", "kafka_connect_"},
		{"jvm_memory_used_bytes", "jvm_memory_"},
		{"node_cpu_seconds_total", "node_cpu_"},
		{"simple", "simple_"},
	}

	for _, tt := range tests {
		got := metricPrefix(tt.name)
		if got != tt.want {
			t.Errorf("metricPrefix(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestGroupDisplayName(t *testing.T) {
	tests := []struct {
		prefix string
		want   string
	}{
		{"debezium_streaming_", "Debezium Streaming"},
		{"kafka_connect_", "Kafka Connect"},
		{"jvm_memory_", "Jvm Memory"},
		{"process_resident_", "Process Resident"},
	}

	for _, tt := range tests {
		got := groupDisplayName(tt.prefix)
		if got != tt.want {
			t.Errorf("groupDisplayName(%q) = %q, want %q", tt.prefix, got, tt.want)
		}
	}
}
