package metrics

import (
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"time"
)

// MockHandler serves diverse Prometheus-format metrics for development and testing.
func MockHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		now := float64(time.Now().UnixMilli()) / 1000.0
		sin := math.Sin(now / 60.0)
		noise := rand.Float64() * 0.1

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		// Kafka broker metrics (gauge)
		fmt.Fprintf(w, "# HELP kafka_server_brokertopicmetrics_bytesinpersec Bytes in per second\n")
		fmt.Fprintf(w, "# TYPE kafka_server_brokertopicmetrics_bytesinpersec gauge\n")
		fmt.Fprintf(w, "kafka_server_brokertopicmetrics_bytesinpersec{topic=\"\"} %f\n", 100000+sin*50000+noise*10000)

		fmt.Fprintf(w, "# HELP kafka_server_brokertopicmetrics_bytesoutpersec Bytes out per second\n")
		fmt.Fprintf(w, "# TYPE kafka_server_brokertopicmetrics_bytesoutpersec gauge\n")
		fmt.Fprintf(w, "kafka_server_brokertopicmetrics_bytesoutpersec{topic=\"\"} %f\n", 70000+sin*30000+noise*8000)

		fmt.Fprintf(w, "# HELP kafka_server_brokertopicmetrics_messagesinpersec Messages in per second\n")
		fmt.Fprintf(w, "# TYPE kafka_server_brokertopicmetrics_messagesinpersec gauge\n")
		fmt.Fprintf(w, "kafka_server_brokertopicmetrics_messagesinpersec{topic=\"\"} %f\n", 300+sin*200+noise*50)

		fmt.Fprintf(w, "# HELP kafka_server_replicamanager_underreplicatedpartitions Under-replicated partitions\n")
		fmt.Fprintf(w, "# TYPE kafka_server_replicamanager_underreplicatedpartitions gauge\n")
		fmt.Fprintf(w, "kafka_server_replicamanager_underreplicatedpartitions 0\n")

		fmt.Fprintf(w, "# HELP kafka_controller_kafkacontroller_activecontrollercount Active controller count\n")
		fmt.Fprintf(w, "# TYPE kafka_controller_kafkacontroller_activecontrollercount gauge\n")
		fmt.Fprintf(w, "kafka_controller_kafkacontroller_activecontrollercount 1\n")

		fmt.Fprintf(w, "# HELP kafka_controller_kafkacontroller_offlinepartitionscount Offline partitions count\n")
		fmt.Fprintf(w, "# TYPE kafka_controller_kafkacontroller_offlinepartitionscount gauge\n")
		fmt.Fprintf(w, "kafka_controller_kafkacontroller_offlinepartitionscount 0\n")

		// JVM metrics (gauge, bytes)
		fmt.Fprintf(w, "# HELP jvm_memory_used_bytes JVM memory used\n")
		fmt.Fprintf(w, "# TYPE jvm_memory_used_bytes gauge\n")
		fmt.Fprintf(w, "jvm_memory_used_bytes{area=\"heap\"} %f\n", 600_000_000+sin*100_000_000+noise*20_000_000)
		fmt.Fprintf(w, "jvm_memory_used_bytes{area=\"nonheap\"} %f\n", 100_000_000+sin*20_000_000+noise*5_000_000)

		fmt.Fprintf(w, "# HELP jvm_memory_max_bytes JVM memory max\n")
		fmt.Fprintf(w, "# TYPE jvm_memory_max_bytes gauge\n")
		fmt.Fprintf(w, "jvm_memory_max_bytes{area=\"heap\"} 1073741824\n")

		fmt.Fprintf(w, "# HELP jvm_threads_current Current JVM threads\n")
		fmt.Fprintf(w, "# TYPE jvm_threads_current gauge\n")
		fmt.Fprintf(w, "jvm_threads_current %.0f\n", 65+sin*15+noise*5)

		// App HTTP metrics (gauge)
		fmt.Fprintf(w, "# HELP app_http_requests_total Total HTTP requests\n")
		fmt.Fprintf(w, "# TYPE app_http_requests_total gauge\n")
		fmt.Fprintf(w, "app_http_requests_total{method=\"GET\",status=\"200\"} %f\n", 55000+sin*5000+noise*1000)
		fmt.Fprintf(w, "app_http_requests_total{method=\"POST\",status=\"200\"} %f\n", 13500+sin*1500+noise*300)
		fmt.Fprintf(w, "app_http_requests_total{method=\"GET\",status=\"404\"} %f\n", 250+sin*50+noise*10)

		fmt.Fprintf(w, "# HELP app_http_requests_active Active HTTP requests\n")
		fmt.Fprintf(w, "# TYPE app_http_requests_active gauge\n")
		fmt.Fprintf(w, "app_http_requests_active{method=\"GET\",path=\"/api\"} %f\n", 15+sin*10+noise*3)

		// Request duration (gauge, milliseconds)
		fmt.Fprintf(w, "# HELP app_request_duration_ms Request duration in milliseconds\n")
		fmt.Fprintf(w, "# TYPE app_request_duration_ms gauge\n")
		fmt.Fprintf(w, "app_request_duration_ms{path=\"/api/v1/topics\"} %f\n", 80+sin*70+noise*15)
		fmt.Fprintf(w, "app_request_duration_ms{path=\"/api/v1/messages\"} %f\n", 250+sin*200+noise*40)

		// Process metrics (gauge)
		fmt.Fprintf(w, "# HELP process_resident_memory_bytes Resident memory size in bytes\n")
		fmt.Fprintf(w, "# TYPE process_resident_memory_bytes gauge\n")
		fmt.Fprintf(w, "process_resident_memory_bytes %f\n", 250_000_000+sin*50_000_000+noise*10_000_000)

		fmt.Fprintf(w, "# HELP process_cpu_seconds_total Total user and system CPU time spent in seconds\n")
		fmt.Fprintf(w, "# TYPE process_cpu_seconds_total gauge\n")
		fmt.Fprintf(w, "process_cpu_seconds_total %f\n", 1250+sin*250+noise*50)

		fmt.Fprintf(w, "# HELP process_open_fds Number of open file descriptors\n")
		fmt.Fprintf(w, "# TYPE process_open_fds gauge\n")
		fmt.Fprintf(w, "process_open_fds %.0f\n", 55+sin*25+noise*5)
	}
}
