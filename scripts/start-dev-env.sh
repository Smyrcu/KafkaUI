#!/usr/bin/env bash
# Full dev environment: Kafka + Schema Registry + Kafka Connect + test data.
# Usage: ./scripts/start-dev-env.sh
# Stop:  ./scripts/start-dev-env.sh stop
set -euo pipefail

KB="/opt/kafka/bin"
BS="localhost:9092"
SR="http://localhost:8081"
CT="http://localhost:8083"

# Detect container runtime
if command -v podman &>/dev/null; then
  RUNTIME=podman
elif command -v docker &>/dev/null; then
  RUNTIME=docker
else
  echo "Error: podman or docker required" >&2
  exit 1
fi

if [ "${1:-}" = "stop" ]; then
  echo "Stopping dev environment..."
  $RUNTIME rm -f kafka-dev sr-dev connect-dev 2>/dev/null || true
  echo "Done."
  exit 0
fi

echo "════════════════════════════════════════"
echo " KafkaUI Dev Environment"
echo "════════════════════════════════════════"

# ── Cleanup ─────────────────────────────
$RUNTIME rm -f kafka-dev sr-dev connect-dev 2>/dev/null || true

# ── Kafka ───────────────────────────────
echo "[1/6] Starting Kafka..."
$RUNTIME run -d --name kafka-dev -p 9092:9092 docker.io/apache/kafka:3.9.0 >/dev/null
for i in $(seq 1 30); do
  $RUNTIME exec kafka-dev "$KB/kafka-broker-api-versions.sh" --bootstrap-server "$BS" &>/dev/null && break
  sleep 2
done
if ! $RUNTIME exec kafka-dev "$KB/kafka-broker-api-versions.sh" --bootstrap-server "$BS" &>/dev/null; then
  echo "ERROR: Kafka failed to start" >&2; exit 1
fi
echo "      Kafka ready"

# ── Schema Registry ─────────────────────
echo "[2/6] Starting Schema Registry..."
$RUNTIME run -d --name sr-dev --network host \
  -e SCHEMA_REGISTRY_HOST_NAME=localhost \
  -e SCHEMA_REGISTRY_KAFKASTORE_BOOTSTRAP_SERVERS=$BS \
  -e SCHEMA_REGISTRY_LISTENERS=http://0.0.0.0:8081 \
  docker.io/confluentinc/cp-schema-registry:7.7.1 >/dev/null
for i in $(seq 1 30); do curl -sf "$SR/subjects" &>/dev/null && break; sleep 2; done
if ! curl -sf "$SR/subjects" &>/dev/null; then echo "ERROR: Schema Registry failed to start" >&2; exit 1; fi
echo "      Schema Registry ready (:8081)"

# ── Kafka Connect ───────────────────────
echo "[3/6] Starting Kafka Connect..."
$RUNTIME run -d --name connect-dev --network host \
  -e CONNECT_BOOTSTRAP_SERVERS=$BS \
  -e CONNECT_REST_PORT=8083 \
  -e CONNECT_GROUP_ID=kafkaui-connect \
  -e CONNECT_CONFIG_STORAGE_TOPIC=_connect-configs \
  -e CONNECT_OFFSET_STORAGE_TOPIC=_connect-offsets \
  -e CONNECT_STATUS_STORAGE_TOPIC=_connect-status \
  -e CONNECT_CONFIG_STORAGE_REPLICATION_FACTOR=1 \
  -e CONNECT_OFFSET_STORAGE_REPLICATION_FACTOR=1 \
  -e CONNECT_STATUS_STORAGE_REPLICATION_FACTOR=1 \
  -e CONNECT_KEY_CONVERTER=org.apache.kafka.connect.storage.StringConverter \
  -e CONNECT_VALUE_CONVERTER=org.apache.kafka.connect.json.JsonConverter \
  -e CONNECT_VALUE_CONVERTER_SCHEMAS_ENABLE=false \
  -e CONNECT_REST_ADVERTISED_HOST_NAME=localhost \
  -e CONNECT_PLUGIN_PATH=/usr/share/java \
  docker.io/confluentinc/cp-kafka-connect:7.7.1 >/dev/null
for i in $(seq 1 40); do curl -sf "$CT/" &>/dev/null && break; sleep 3; done
if ! curl -sf "$CT/" &>/dev/null; then echo "ERROR: Kafka Connect failed to start" >&2; exit 1; fi
echo "      Kafka Connect ready (:8083)"

# ── Topics + Messages ───────────────────
echo "[4/6] Creating topics & messages..."
for t in orders users payments notifications audit-log events; do
  p=3; [ "$t" = "events" ] && p=6
  $RUNTIME exec kafka-dev $KB/kafka-topics.sh --bootstrap-server $BS \
    --create --if-not-exists --topic "$t" --partitions $p --replication-factor 1 2>/dev/null
done

for i in $(seq 1 20); do echo "{\"orderId\":$i,\"product\":\"item-$((i%5+1))\",\"amount\":$((RANDOM%1000))}"; done \
  | $RUNTIME exec -i kafka-dev $KB/kafka-console-producer.sh --bootstrap-server $BS --topic orders
for i in $(seq 1 10); do echo "{\"userId\":$i,\"name\":\"user$i\",\"email\":\"u${i}@ex.com\"}"; done \
  | $RUNTIME exec -i kafka-dev $KB/kafka-console-producer.sh --bootstrap-server $BS --topic users
for i in $(seq 1 15); do echo "{\"paymentId\":$i,\"amount\":$((RANDOM%1000))}"; done \
  | $RUNTIME exec -i kafka-dev $KB/kafka-console-producer.sh --bootstrap-server $BS --topic payments
for i in $(seq 1 5); do echo "{\"msg\":\"event $i\"}"; done \
  | $RUNTIME exec -i kafka-dev $KB/kafka-console-producer.sh --bootstrap-server $BS --topic notifications
echo "      6 topics, ~50 messages"

# ── Consumer Groups ─────────────────────
echo "[5/6] Creating consumer groups & users..."
$RUNTIME exec kafka-dev timeout 5 $KB/kafka-console-consumer.sh --bootstrap-server $BS \
  --topic orders --group order-processor --from-beginning --max-messages 10 >/dev/null 2>&1 || true
$RUNTIME exec kafka-dev timeout 5 $KB/kafka-console-consumer.sh --bootstrap-server $BS \
  --topic payments --group payment-service --from-beginning --max-messages 5 >/dev/null 2>&1 || true
$RUNTIME exec kafka-dev timeout 5 $KB/kafka-console-consumer.sh --bootstrap-server $BS \
  --topic users --group user-sync --from-beginning --max-messages 5 >/dev/null 2>&1 || true

$RUNTIME exec kafka-dev $KB/kafka-configs.sh --bootstrap-server $BS \
  --alter --add-config 'SCRAM-SHA-256=[iterations=4096,password=alice]' --entity-type users --entity-name alice 2>/dev/null || true
$RUNTIME exec kafka-dev $KB/kafka-configs.sh --bootstrap-server $BS \
  --alter --add-config 'SCRAM-SHA-256=[iterations=4096,password=bob]' --entity-type users --entity-name bob 2>/dev/null || true
$RUNTIME exec kafka-dev $KB/kafka-configs.sh --bootstrap-server $BS \
  --alter --add-config 'SCRAM-SHA-512=[iterations=8192,password=admin]' --entity-type users --entity-name admin 2>/dev/null || true
echo "      3 groups, 3 SCRAM users"

# ── Schemas + Connectors ────────────────
echo "[6/6] Registering schemas & connectors..."
curl -sf -X POST "$SR/subjects/orders-value/versions" -H "Content-Type: application/vnd.schemaregistry.v1+json" \
  -d '{"schemaType":"AVRO","schema":"{\"type\":\"record\",\"name\":\"Order\",\"fields\":[{\"name\":\"orderId\",\"type\":\"int\"},{\"name\":\"product\",\"type\":\"string\"},{\"name\":\"amount\",\"type\":\"int\"}]}"}' >/dev/null
curl -sf -X POST "$SR/subjects/users-value/versions" -H "Content-Type: application/vnd.schemaregistry.v1+json" \
  -d '{"schemaType":"JSON","schema":"{\"type\":\"object\",\"properties\":{\"userId\":{\"type\":\"integer\"},\"name\":{\"type\":\"string\"}}}"}' >/dev/null
curl -sf -X POST "$SR/subjects/payments-value/versions" -H "Content-Type: application/vnd.schemaregistry.v1+json" \
  -d '{"schemaType":"AVRO","schema":"{\"type\":\"record\",\"name\":\"Payment\",\"fields\":[{\"name\":\"paymentId\",\"type\":\"int\"},{\"name\":\"amount\",\"type\":\"int\"}]}"}' >/dev/null

curl -sf -X POST "$CT/connectors" -H "Content-Type: application/json" \
  -d '{"name":"file-sink-demo","config":{"connector.class":"org.apache.kafka.connect.file.FileStreamSinkConnector","tasks.max":"1","file":"/tmp/output.txt","topics":"orders"}}' >/dev/null 2>&1 || true
curl -sf -X POST "$CT/connectors" -H "Content-Type: application/json" \
  -d '{"name":"file-source-demo","config":{"connector.class":"org.apache.kafka.connect.file.FileStreamSourceConnector","tasks.max":"1","file":"/tmp/input.txt","topic":"file-source-output"}}' >/dev/null 2>&1 || true
echo "      3 schemas, 2 connectors"

# ── Config ──────────────────────────────
if [ -f config.yaml ]; then
  cp config.yaml "config.yaml.bak.$(date +%s)"
fi
cat > config.yaml <<'YAML'
server:
  port: 8080
  base-path: ""

auth:
  enabled: false

clusters:
  - name: local
    bootstrap-servers: localhost:9092
    schema-registry:
      url: http://localhost:8081
    kafka-connect:
      - name: local-connect
        url: http://localhost:8083
YAML

echo ""
echo "════════════════════════════════════════"
echo " Ready! Run: make dev"
echo " UI: http://localhost:5173"
echo "════════════════════════════════════════"
echo " Topics:     orders(20) users(10) payments(15) notifications(5) audit-log events"
echo " Groups:     order-processor payment-service user-sync"
echo " Users:      alice bob admin"
echo " Schemas:    orders-value users-value payments-value"
echo " Connectors: file-sink-demo file-source-demo"
echo " Stop:       ./scripts/start-dev-env.sh stop"
echo "════════════════════════════════════════"
