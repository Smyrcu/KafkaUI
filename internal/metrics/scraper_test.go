package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const sampleMixedMetrics = `
# HELP kafka_server_brokertopicmetrics_bytesinpersec Bytes in
# TYPE kafka_server_brokertopicmetrics_bytesinpersec gauge
kafka_server_brokertopicmetrics_bytesinpersec{topic=""} 1234.5
# HELP debezium_streaming_connected CDC connected
# TYPE debezium_streaming_connected untyped
debezium_streaming_connected{plugin="postgres",server="supplier"} 1.0
# HELP kafka_connect_sink_task_sink_record_read_rate Records read rate
# TYPE kafka_connect_sink_task_sink_record_read_rate untyped
kafka_connect_sink_task_sink_record_read_rate{connector="my-sink",task="0"} 42.5
# HELP jvm_memory_used_bytes JVM memory used
# TYPE jvm_memory_used_bytes gauge
jvm_memory_used_bytes{area="heap"} 500000000
`

func TestScraper_ScrapeGeneric(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(sampleMixedMetrics))
	}))
	defer srv.Close()

	s := NewScraper(srv.URL)
	snap, err := s.Scrape(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(snap) != 4 {
		t.Errorf("expected 4 metric families, got %d", len(snap))
	}

	// Check debezium metric with labels
	dbz, ok := snap["debezium_streaming_connected"]
	if !ok {
		t.Fatal("missing debezium_streaming_connected")
	}
	if dbz.Type != "untyped" {
		t.Errorf("expected type untyped, got %s", dbz.Type)
	}
	if len(dbz.Samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(dbz.Samples))
	}
	if dbz.Samples[0].Value != 1.0 {
		t.Errorf("expected value 1.0, got %f", dbz.Samples[0].Value)
	}
	if dbz.Samples[0].Labels["server"] != "supplier" {
		t.Errorf("expected label server=supplier, got %v", dbz.Samples[0].Labels)
	}
	if dbz.Samples[0].Labels["plugin"] != "postgres" {
		t.Errorf("expected label plugin=postgres, got %v", dbz.Samples[0].Labels)
	}

	// Check kafka connect metric
	kc, ok := snap["kafka_connect_sink_task_sink_record_read_rate"]
	if !ok {
		t.Fatal("missing kafka_connect metric")
	}
	if kc.Samples[0].Value != 42.5 {
		t.Errorf("expected 42.5, got %f", kc.Samples[0].Value)
	}

	// Check gauge metric
	jvm, ok := snap["jvm_memory_used_bytes"]
	if !ok {
		t.Fatal("missing jvm_memory_used_bytes")
	}
	if jvm.Type != "gauge" {
		t.Errorf("expected type gauge, got %s", jvm.Type)
	}
	if jvm.Samples[0].Value != 500000000 {
		t.Errorf("expected 500000000, got %f", jvm.Samples[0].Value)
	}

	// Check kafka broker metric with empty topic label
	broker, ok := snap["kafka_server_brokertopicmetrics_bytesinpersec"]
	if !ok {
		t.Fatal("missing kafka_server metric")
	}
	if broker.Samples[0].Value != 1234.5 {
		t.Errorf("expected 1234.5, got %f", broker.Samples[0].Value)
	}
}

func TestScraper_ScrapeError(t *testing.T) {
	s := NewScraper("http://localhost:1/metrics")
	_, err := s.Scrape(context.Background())
	if err == nil {
		t.Fatal("expected error for unreachable host")
	}
}

func TestScraper_ScrapeNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	s := NewScraper(srv.URL)
	_, err := s.Scrape(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

const counterMetrics = `
# HELP http_requests_total Total HTTP requests
# TYPE http_requests_total counter
http_requests_total{method="GET",status="200"} 50000
http_requests_total{method="POST",status="200"} 12000
`

func TestScraper_CounterType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(counterMetrics))
	}))
	defer srv.Close()

	s := NewScraper(srv.URL)
	snap, err := s.Scrape(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fam, ok := snap["http_requests_total"]
	if !ok {
		t.Fatal("missing http_requests_total")
	}
	if fam.Type != "counter" {
		t.Errorf("expected type counter, got %s", fam.Type)
	}
	if len(fam.Samples) != 2 {
		t.Errorf("expected 2 samples, got %d", len(fam.Samples))
	}
}
