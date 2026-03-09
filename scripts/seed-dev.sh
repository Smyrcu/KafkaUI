#!/usr/bin/env bash
# Seed the dev environment with test data for manual testing.
# Requires: docker/podman compose running (docker-compose.dev.yaml)
set -euo pipefail

KAFKA="docker compose -f docker-compose.dev.yaml exec -T kafka"
KAFKA_BIN="/opt/kafka/bin"
BOOTSTRAP="localhost:9092"
SR_URL="http://localhost:8081"
CONNECT_URL="http://localhost:8083"

echo "⏳ Waiting for Kafka..."
until $KAFKA $KAFKA_BIN/kafka-broker-api-versions.sh --bootstrap-server $BOOTSTRAP &>/dev/null; do sleep 1; done
echo "✅ Kafka ready"

# ── Topics ──────────────────────────────────────────────
echo "📦 Creating topics..."
for topic in orders users payments notifications audit-log; do
  $KAFKA $KAFKA_BIN/kafka-topics.sh --bootstrap-server $BOOTSTRAP \
    --create --if-not-exists --topic "$topic" --partitions 3 --replication-factor 1
done
$KAFKA $KAFKA_BIN/kafka-topics.sh --bootstrap-server $BOOTSTRAP \
  --create --if-not-exists --topic events --partitions 6 --replication-factor 1

# ── Messages ────────────────────────────────────────────
echo "📨 Producing test messages..."
for i in $(seq 1 20); do
  echo "{\"orderId\":$i,\"product\":\"item-$((i%5+1))\",\"amount\":$((RANDOM%1000)),\"ts\":\"2026-03-09T10:${i}:00Z\"}"
done | $KAFKA $KAFKA_BIN/kafka-console-producer.sh --bootstrap-server $BOOTSTRAP --topic orders

for i in $(seq 1 10); do
  echo "{\"userId\":$i,\"name\":\"user$i\",\"email\":\"user${i}@example.com\",\"role\":\"$([ $((i%3)) -eq 0 ] && echo admin || echo viewer)\"}"
done | $KAFKA $KAFKA_BIN/kafka-console-producer.sh --bootstrap-server $BOOTSTRAP --topic users

for i in $(seq 1 15); do
  echo "{\"paymentId\":$i,\"orderId\":$((i%20+1)),\"amount\":$((RANDOM%1000)).99,\"status\":\"$([ $((i%4)) -eq 0 ] && echo failed || echo completed)\"}"
done | $KAFKA $KAFKA_BIN/kafka-console-producer.sh --bootstrap-server $BOOTSTRAP --topic payments

for i in $(seq 1 5); do
  echo "{\"type\":\"info\",\"message\":\"System event $i\",\"source\":\"service-$((i%3+1))\"}"
done | $KAFKA $KAFKA_BIN/kafka-console-producer.sh --bootstrap-server $BOOTSTRAP --topic notifications

# ── Consumer Groups (read some messages to create groups) ──
echo "👥 Creating consumer groups..."
$KAFKA timeout 5 $KAFKA_BIN/kafka-console-consumer.sh --bootstrap-server $BOOTSTRAP \
  --topic orders --group order-processor --from-beginning --max-messages 10 >/dev/null 2>&1 || true
$KAFKA timeout 5 $KAFKA_BIN/kafka-console-consumer.sh --bootstrap-server $BOOTSTRAP \
  --topic payments --group payment-service --from-beginning --max-messages 5 >/dev/null 2>&1 || true
$KAFKA timeout 5 $KAFKA_BIN/kafka-console-consumer.sh --bootstrap-server $BOOTSTRAP \
  --topic users --group user-sync --from-beginning --max-messages 5 >/dev/null 2>&1 || true

# ── SCRAM Users ─────────────────────────────────────────
echo "👤 Creating SCRAM users..."
$KAFKA $KAFKA_BIN/kafka-configs.sh --bootstrap-server $BOOTSTRAP \
  --alter --add-config 'SCRAM-SHA-256=[iterations=4096,password=alice-secret]' \
  --entity-type users --entity-name alice 2>/dev/null || true
$KAFKA $KAFKA_BIN/kafka-configs.sh --bootstrap-server $BOOTSTRAP \
  --alter --add-config 'SCRAM-SHA-256=[iterations=4096,password=bob-secret]' \
  --entity-type users --entity-name bob 2>/dev/null || true
$KAFKA $KAFKA_BIN/kafka-configs.sh --bootstrap-server $BOOTSTRAP \
  --alter --add-config 'SCRAM-SHA-512=[iterations=8192,password=admin-secret]' \
  --entity-type users --entity-name admin 2>/dev/null || true

# ── ACLs ────────────────────────────────────────────────
echo "🔒 Creating ACLs..."
$KAFKA $KAFKA_BIN/kafka-acls.sh --bootstrap-server $BOOTSTRAP \
  --add --allow-principal User:alice --operation Read --operation Describe --topic orders --group order-processor 2>/dev/null || true
$KAFKA $KAFKA_BIN/kafka-acls.sh --bootstrap-server $BOOTSTRAP \
  --add --allow-principal User:bob --operation Read --operation Write --topic payments 2>/dev/null || true
$KAFKA $KAFKA_BIN/kafka-acls.sh --bootstrap-server $BOOTSTRAP \
  --add --deny-principal User:bob --operation Delete --topic '*' 2>/dev/null || true
$KAFKA $KAFKA_BIN/kafka-acls.sh --bootstrap-server $BOOTSTRAP \
  --add --allow-principal User:admin --operation All --topic '*' --group '*' --cluster 2>/dev/null || true

# ── Schema Registry ─────────────────────────────────────
echo "⏳ Waiting for Schema Registry..."
until curl -sf "$SR_URL/subjects" &>/dev/null; do sleep 1; done
echo "✅ Schema Registry ready"

echo "📋 Registering schemas..."
curl -sf -X POST "$SR_URL/subjects/orders-value/versions" \
  -H "Content-Type: application/vnd.schemaregistry.v1+json" \
  -d '{
    "schemaType": "AVRO",
    "schema": "{\"type\":\"record\",\"name\":\"Order\",\"namespace\":\"com.example\",\"fields\":[{\"name\":\"orderId\",\"type\":\"int\"},{\"name\":\"product\",\"type\":\"string\"},{\"name\":\"amount\",\"type\":\"int\"},{\"name\":\"ts\",\"type\":\"string\"}]}"
  }' >/dev/null

curl -sf -X POST "$SR_URL/subjects/users-value/versions" \
  -H "Content-Type: application/vnd.schemaregistry.v1+json" \
  -d '{
    "schemaType": "JSON",
    "schema": "{\"type\":\"object\",\"properties\":{\"userId\":{\"type\":\"integer\"},\"name\":{\"type\":\"string\"},\"email\":{\"type\":\"string\"},\"role\":{\"type\":\"string\"}},\"required\":[\"userId\",\"name\"]}"
  }' >/dev/null

curl -sf -X POST "$SR_URL/subjects/payments-value/versions" \
  -H "Content-Type: application/vnd.schemaregistry.v1+json" \
  -d '{
    "schemaType": "AVRO",
    "schema": "{\"type\":\"record\",\"name\":\"Payment\",\"namespace\":\"com.example\",\"fields\":[{\"name\":\"paymentId\",\"type\":\"int\"},{\"name\":\"orderId\",\"type\":\"int\"},{\"name\":\"amount\",\"type\":\"string\"},{\"name\":\"status\",\"type\":\"string\"}]}"
  }' >/dev/null

# ── Kafka Connect ───────────────────────────────────────
echo "⏳ Waiting for Kafka Connect..."
until curl -sf "$CONNECT_URL/" &>/dev/null; do sleep 2; done
echo "✅ Kafka Connect ready"

echo "🔌 Creating connectors..."
# FileStream source: generates data from a file into a topic
curl -sf -X POST "$CONNECT_URL/connectors" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "file-source-demo",
    "config": {
      "connector.class": "org.apache.kafka.connect.file.FileStreamSourceConnector",
      "tasks.max": "1",
      "file": "/tmp/connect-demo-input.txt",
      "topic": "file-source-output"
    }
  }' >/dev/null 2>&1 || true

# FileStream sink: writes topic data to a file
curl -sf -X POST "$CONNECT_URL/connectors" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "file-sink-demo",
    "config": {
      "connector.class": "org.apache.kafka.connect.file.FileStreamSinkConnector",
      "tasks.max": "1",
      "file": "/tmp/connect-demo-output.txt",
      "topics": "orders"
    }
  }' >/dev/null 2>&1 || true

echo ""
echo "🎉 Dev environment ready! Test data:"
echo "   Topics:          orders, users, payments, notifications, audit-log, events"
echo "   Messages:        ~50 across orders/users/payments/notifications"
echo "   Consumer Groups: order-processor, payment-service, user-sync"
echo "   SCRAM Users:     alice (SHA-256), bob (SHA-256), admin (SHA-512)"
echo "   ACLs:            4 rules for alice, bob, admin"
echo "   Schemas:         orders-value (AVRO), users-value (JSON), payments-value (AVRO)"
echo "   Connectors:      file-source-demo, file-sink-demo"
echo "   Metrics:         http://localhost:9404/metrics"
echo ""
echo "Run 'make dev' to start the UI."
