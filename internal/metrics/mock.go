package metrics

import (
	"fmt"
	"math/rand/v2"
	"net/http"
)

// MockHandler serves diverse Prometheus-format metrics for development and testing.
func MockHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		// Kafka broker metrics (gauge)
		fmt.Fprintf(w, "# HELP kafka_server_brokertopicmetrics_bytesinpersec Bytes in per second\n")
		fmt.Fprintf(w, "# TYPE kafka_server_brokertopicmetrics_bytesinpersec gauge\n")
		fmt.Fprintf(w, "kafka_server_brokertopicmetrics_bytesinpersec{topic=\"\"} %f\n", 50000+rand.Float64()*100000)

		fmt.Fprintf(w, "# HELP kafka_server_brokertopicmetrics_bytesoutpersec Bytes out per second\n")
		fmt.Fprintf(w, "# TYPE kafka_server_brokertopicmetrics_bytesoutpersec gauge\n")
		fmt.Fprintf(w, "kafka_server_brokertopicmetrics_bytesoutpersec{topic=\"\"} %f\n", 30000+rand.Float64()*80000)

		fmt.Fprintf(w, "# HELP kafka_server_brokertopicmetrics_messagesinpersec Messages in per second\n")
		fmt.Fprintf(w, "# TYPE kafka_server_brokertopicmetrics_messagesinpersec gauge\n")
		fmt.Fprintf(w, "kafka_server_brokertopicmetrics_messagesinpersec{topic=\"\"} %f\n", 100+rand.Float64()*500)

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
		fmt.Fprintf(w, "jvm_memory_used_bytes{area=\"heap\"} %f\n", 500_000_000+rand.Float64()*200_000_000)
		fmt.Fprintf(w, "jvm_memory_used_bytes{area=\"nonheap\"} %f\n", 80_000_000+rand.Float64()*40_000_000)

		fmt.Fprintf(w, "# HELP jvm_memory_max_bytes JVM memory max\n")
		fmt.Fprintf(w, "# TYPE jvm_memory_max_bytes gauge\n")
		fmt.Fprintf(w, "jvm_memory_max_bytes{area=\"heap\"} 1073741824\n")

		fmt.Fprintf(w, "# HELP jvm_threads_current Current JVM threads\n")
		fmt.Fprintf(w, "# TYPE jvm_threads_current gauge\n")
		fmt.Fprintf(w, "jvm_threads_current %f\n", 50+rand.Float64()*30)

		// App HTTP metrics (counter + gauge)
		fmt.Fprintf(w, "# HELP app_http_requests_total Total HTTP requests\n")
		fmt.Fprintf(w, "# TYPE app_http_requests_total counter\n")
		fmt.Fprintf(w, "app_http_requests_total{method=\"GET\",status=\"200\"} %f\n", 50000+rand.Float64()*10000)
		fmt.Fprintf(w, "app_http_requests_total{method=\"POST\",status=\"200\"} %f\n", 12000+rand.Float64()*3000)
		fmt.Fprintf(w, "app_http_requests_total{method=\"GET\",status=\"404\"} %f\n", 200+rand.Float64()*100)

		fmt.Fprintf(w, "# HELP app_http_requests_active Active HTTP requests\n")
		fmt.Fprintf(w, "# TYPE app_http_requests_active gauge\n")
		fmt.Fprintf(w, "app_http_requests_active{method=\"GET\",path=\"/api\"} %f\n", 5+rand.Float64()*20)

		// Request duration (gauge, milliseconds)
		fmt.Fprintf(w, "# HELP app_request_duration_ms Request duration in milliseconds\n")
		fmt.Fprintf(w, "# TYPE app_request_duration_ms gauge\n")
		fmt.Fprintf(w, "app_request_duration_ms{path=\"/api/v1/topics\"} %f\n", 10+rand.Float64()*150)
		fmt.Fprintf(w, "app_request_duration_ms{path=\"/api/v1/messages\"} %f\n", 50+rand.Float64()*400)

		// Process metrics (gauge)
		fmt.Fprintf(w, "# HELP process_resident_memory_bytes Resident memory size in bytes\n")
		fmt.Fprintf(w, "# TYPE process_resident_memory_bytes gauge\n")
		fmt.Fprintf(w, "process_resident_memory_bytes %f\n", 200_000_000+rand.Float64()*100_000_000)

		fmt.Fprintf(w, "# HELP process_cpu_seconds_total Total user and system CPU time spent in seconds\n")
		fmt.Fprintf(w, "# TYPE process_cpu_seconds_total counter\n")
		fmt.Fprintf(w, "process_cpu_seconds_total %f\n", 1000+rand.Float64()*500)

		fmt.Fprintf(w, "# HELP process_open_fds Number of open file descriptors\n")
		fmt.Fprintf(w, "# TYPE process_open_fds gauge\n")
		fmt.Fprintf(w, "process_open_fds %f\n", 30+rand.Float64()*50)
	}
}
