package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const sampleMetrics = `
# HELP kafka_server_brokertopicmetrics_bytesinpersec
# TYPE kafka_server_brokertopicmetrics_bytesinpersec gauge
kafka_server_brokertopicmetrics_bytesinpersec{topic="",} 1234.5
# HELP kafka_server_brokertopicmetrics_bytesoutpersec
# TYPE kafka_server_brokertopicmetrics_bytesoutpersec gauge
kafka_server_brokertopicmetrics_bytesoutpersec{topic="",} 5678.9
# HELP kafka_server_brokertopicmetrics_messagesinpersec
# TYPE kafka_server_brokertopicmetrics_messagesinpersec gauge
kafka_server_brokertopicmetrics_messagesinpersec{topic="",} 42.0
# HELP kafka_server_replicamanager_underreplicatedpartitions
# TYPE kafka_server_replicamanager_underreplicatedpartitions gauge
kafka_server_replicamanager_underreplicatedpartitions 0
# HELP kafka_controller_kafkacontroller_activecontrollercount
# TYPE kafka_controller_kafkacontroller_activecontrollercount gauge
kafka_controller_kafkacontroller_activecontrollercount 1
# HELP kafka_controller_kafkacontroller_offlinepartitionscount
# TYPE kafka_controller_kafkacontroller_offlinepartitionscount gauge
kafka_controller_kafkacontroller_offlinepartitionscount 0
`

func TestScraper_Scrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(sampleMetrics))
	}))
	defer srv.Close()

	s := NewScraper(srv.URL + "/metrics")
	m, err := s.Scrape(context.Background(), srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.BytesInPerSec != 1234.5 {
		t.Errorf("expected BytesInPerSec 1234.5, got %f", m.BytesInPerSec)
	}
	if m.BytesOutPerSec != 5678.9 {
		t.Errorf("expected BytesOutPerSec 5678.9, got %f", m.BytesOutPerSec)
	}
	if m.MessagesInPerSec != 42.0 {
		t.Errorf("expected MessagesInPerSec 42.0, got %f", m.MessagesInPerSec)
	}
	if m.UnderReplicatedPartitions != 0 {
		t.Errorf("expected UnderReplicatedPartitions 0, got %f", m.UnderReplicatedPartitions)
	}
	if m.ActiveControllerCount != 1 {
		t.Errorf("expected ActiveControllerCount 1, got %f", m.ActiveControllerCount)
	}
	if m.OfflinePartitionsCount != 0 {
		t.Errorf("expected OfflinePartitionsCount 0, got %f", m.OfflinePartitionsCount)
	}
}

func TestScraper_ScrapeError(t *testing.T) {
	s := NewScraper("http://localhost:1/metrics")
	_, err := s.Scrape(context.Background(), "localhost:1")
	if err == nil {
		t.Fatal("expected error for unreachable host")
	}
}

func TestScraper_BuildURL(t *testing.T) {
	s := NewScraper("http://{host}/metrics")
	url := s.buildURL("kafka-1:9092")
	if url != "http://kafka-1:9092/metrics" {
		t.Errorf("expected http://kafka-1:9092/metrics, got %s", url)
	}
}
