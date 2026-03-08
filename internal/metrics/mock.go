package metrics

import (
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"time"
)

// MockHandler serves fake Prometheus metrics that look realistic.
// Designed for dev/testing when no real JMX exporter is available.
type MockHandler struct{}

func NewMockHandler() *MockHandler {
	return &MockHandler{}
}

func (h *MockHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	now := float64(time.Now().UnixMilli()) / 1000.0
	sin := math.Sin(now / 60.0)

	bytesIn := 5_000_000 + sin*2_000_000 + rand.Float64()*500_000
	bytesOut := 8_000_000 + sin*3_000_000 + rand.Float64()*800_000
	messagesIn := 1200 + sin*400 + rand.Float64()*100
	underReplicated := 0.0
	if rand.IntN(50) == 0 {
		underReplicated = float64(rand.IntN(3) + 1)
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	fmt.Fprintf(w, "# HELP kafka_server_brokertopicmetrics_bytesinpersec Bytes in per second\n")
	fmt.Fprintf(w, "# TYPE kafka_server_brokertopicmetrics_bytesinpersec gauge\n")
	fmt.Fprintf(w, "kafka_server_brokertopicmetrics_bytesinpersec{topic=\"\"} %f\n\n", bytesIn)

	fmt.Fprintf(w, "# HELP kafka_server_brokertopicmetrics_bytesoutpersec Bytes out per second\n")
	fmt.Fprintf(w, "# TYPE kafka_server_brokertopicmetrics_bytesoutpersec gauge\n")
	fmt.Fprintf(w, "kafka_server_brokertopicmetrics_bytesoutpersec{topic=\"\"} %f\n\n", bytesOut)

	fmt.Fprintf(w, "# HELP kafka_server_brokertopicmetrics_messagesinpersec Messages in per second\n")
	fmt.Fprintf(w, "# TYPE kafka_server_brokertopicmetrics_messagesinpersec gauge\n")
	fmt.Fprintf(w, "kafka_server_brokertopicmetrics_messagesinpersec{topic=\"\"} %f\n\n", messagesIn)

	fmt.Fprintf(w, "# HELP kafka_server_replicamanager_underreplicatedpartitions Under-replicated partitions\n")
	fmt.Fprintf(w, "# TYPE kafka_server_replicamanager_underreplicatedpartitions gauge\n")
	fmt.Fprintf(w, "kafka_server_replicamanager_underreplicatedpartitions %f\n\n", underReplicated)

	fmt.Fprintf(w, "# HELP kafka_controller_kafkacontroller_activecontrollercount Active controller count\n")
	fmt.Fprintf(w, "# TYPE kafka_controller_kafkacontroller_activecontrollercount gauge\n")
	fmt.Fprintf(w, "kafka_controller_kafkacontroller_activecontrollercount 1\n\n")

	fmt.Fprintf(w, "# HELP kafka_controller_kafkacontroller_offlinepartitionscount Offline partitions count\n")
	fmt.Fprintf(w, "# TYPE kafka_controller_kafkacontroller_offlinepartitionscount gauge\n")
	fmt.Fprintf(w, "kafka_controller_kafkacontroller_offlinepartitionscount 0\n")
}
