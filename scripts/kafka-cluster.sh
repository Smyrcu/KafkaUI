#!/usr/bin/env bash
set -euo pipefail

CLUSTER_ID="MkU3OEVBNTcwNTJENDM2Qk"
KAFKA_IMAGE="docker.io/apache/kafka:3.9.0"
NETWORK="kafkanet"
NODES=3

# Detect container runtime
if command -v docker &>/dev/null; then
  CTR=docker
elif command -v podman &>/dev/null; then
  CTR=podman
else
  echo "Error: docker or podman required" >&2
  exit 1
fi

usage() {
  echo "Usage: $0 {up|down|status|logs}"
  echo
  echo "  up      Start 3-node Kafka HA cluster (KRaft)"
  echo "  down    Stop and remove all Kafka containers"
  echo "  status  Show container status"
  echo "  logs    Tail logs from all nodes"
  exit 1
}

cmd_up() {
  echo "Starting ${NODES}-node Kafka HA cluster..."

  # Create network if needed
  $CTR network inspect "$NETWORK" &>/dev/null 2>&1 || $CTR network create "$NETWORK"

  VOTERS=""
  for i in $(seq 1 $NODES); do
    [ -n "$VOTERS" ] && VOTERS="${VOTERS},"
    VOTERS="${VOTERS}${i}@kafka${i}:9093"
  done

  for i in $(seq 1 $NODES); do
    PORT=$((i * 10000 + 9092))
    NAME="kafka${i}"

    # Skip if already running
    if $CTR ps --format '{{.Names}}' 2>/dev/null | grep -qw "$NAME"; then
      echo "  $NAME already running (port $PORT)"
      continue
    fi

    # Remove stopped container if exists
    $CTR rm -f "$NAME" &>/dev/null 2>&1 || true

    $CTR run -d \
      --name "$NAME" \
      --network "$NETWORK" \
      --hostname "$NAME" \
      -p "${PORT}:${PORT}" \
      -e KAFKA_NODE_ID="$i" \
      -e KAFKA_PROCESS_ROLES=broker,controller \
      -e "KAFKA_LISTENERS=PLAINTEXT://:9092,CONTROLLER://:9093,EXTERNAL://:${PORT}" \
      -e "KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://${NAME}:9092,EXTERNAL://localhost:${PORT}" \
      -e KAFKA_CONTROLLER_LISTENER_NAMES=CONTROLLER \
      -e "KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT,EXTERNAL:PLAINTEXT" \
      -e KAFKA_INTER_BROKER_LISTENER_NAME=PLAINTEXT \
      -e "KAFKA_CONTROLLER_QUORUM_VOTERS=${VOTERS}" \
      -e KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=3 \
      -e KAFKA_DEFAULT_REPLICATION_FACTOR=3 \
      -e KAFKA_MIN_INSYNC_REPLICAS=2 \
      -e KAFKA_LOG_DIRS=/tmp/kraft-combined-logs \
      -e "CLUSTER_ID=${CLUSTER_ID}" \
      "$KAFKA_IMAGE" >/dev/null

    echo "  $NAME started (port $PORT)"
  done

  echo
  echo "Waiting for cluster quorum..."
  sleep 15

  # Verify
  READY=0
  for i in $(seq 1 $NODES); do
    if $CTR logs "kafka${i}" 2>&1 | grep -q "Kafka Server started"; then
      READY=$((READY + 1))
    fi
  done

  if [ "$READY" -eq "$NODES" ]; then
    echo "Cluster ready! All ${NODES} nodes running."
  else
    echo "Warning: only ${READY}/${NODES} nodes started. Check logs with: $0 logs"
  fi

  echo
  echo "Bootstrap servers: localhost:19092,localhost:29092,localhost:39092"
  echo "Config: RF=3, min.insync.replicas=2, KRaft mode"
}

cmd_down() {
  echo "Stopping Kafka cluster..."
  for i in $(seq 1 $NODES); do
    $CTR rm -f "kafka${i}" &>/dev/null 2>&1 && echo "  kafka${i} removed" || true
  done
  $CTR network rm "$NETWORK" &>/dev/null 2>&1 && echo "  network removed" || true
  echo "Done."
}

cmd_status() {
  for i in $(seq 1 $NODES); do
    NAME="kafka${i}"
    if $CTR ps --format '{{.Names}}' 2>/dev/null | grep -qw "$NAME"; then
      PORT=$((i * 10000 + 9092))
      echo "  $NAME: running (localhost:$PORT)"
    else
      echo "  $NAME: stopped"
    fi
  done
}

cmd_logs() {
  for i in $(seq 1 $NODES); do
    echo "=== kafka${i} ==="
    $CTR logs --tail 5 "kafka${i}" 2>&1
    echo
  done
}

case "${1:-}" in
  up)     cmd_up ;;
  down)   cmd_down ;;
  status) cmd_status ;;
  logs)   cmd_logs ;;
  *)      usage ;;
esac
