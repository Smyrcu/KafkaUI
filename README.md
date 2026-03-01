# KafkaUI

A modern web UI for Apache Kafka. Go backend with embedded React frontend, shipped as a single binary.

## Features

- Cluster management (multi-cluster support)
- Broker information and monitoring
- Topic CRUD (create, configure, delete)
- Message browsing with filters (partition, offset, timestamp)
- Message producing
- Live tail via WebSocket (real-time message streaming)
- Consumer Groups (list, details, reset offsets)
- Schema Registry integration (list, create, delete schemas -- AVRO, JSON, Protobuf)
- Kafka Connect management (CRUD, pause/resume/restart connectors)
- KSQL query execution with quick actions
- ACL management (create, list, delete access control entries)
- Dashboard with cluster overview and auto-refresh
- Authentication: SASL (PLAIN, SCRAM-SHA-256/512), TLS/SSL
- OIDC authentication with RBAC
- Data masking engine for sensitive fields
- Dark/Light theme
- Swagger UI + OpenAPI spec
- Single binary deployment with Docker support

## Tech Stack

- **Backend**: Go 1.25, chi/v5 router, franz-go Kafka client, gorilla/websocket
- **Frontend**: React 19, TypeScript, Vite, shadcn/ui, Tailwind CSS v4, TanStack Query
- **Auth**: OIDC, RBAC, session-based with HMAC signing

## Quick Start

### Docker Compose

The fastest way to get started. Launches Kafka and KafkaUI together, preconfigured:

```bash
docker compose up --build
```

Open http://localhost:8080.

### Standalone Binary

```bash
# Build
make build

# Run
./kafkaui --config config.yaml
```

### Docker Image

```bash
# Build image
make docker

# Run with custom config
docker run -p 8080:8080 -v $(pwd)/config.yaml:/etc/kafkaui/config.yaml kafkaui
```

## Development

### Prerequisites

- Go 1.25+
- Node.js 22+
- A running Kafka instance (or use `docker compose up kafka` to start one)

### Setup

```bash
cd frontend && npm install && cd ..
cp config.example.yaml config.yaml
```

Edit `config.yaml` to point to your Kafka broker(s).

### Commands

| Command | Description |
|---|---|
| `make dev` | Concurrent hot-reload (Go backend + Vite frontend) |
| `make build` | Production build (single binary with embedded frontend) |
| `make test` | Run all tests (backend + frontend) |
| `make docker` | Build Docker image |
| `make clean` | Remove build artifacts |

### Dev URLs

- Frontend (Vite HMR): http://localhost:5173
- Backend API: http://localhost:8080
- Swagger UI: http://localhost:8080/api/v1/docs

## Configuration

Configuration is defined in YAML. Environment variables are supported with `${VAR_NAME}` syntax.

```yaml
server:
  port: 8080
  base-path: ""

auth:
  enabled: true
  type: oidc
  oidc:
    issuer: https://accounts.example.com
    client-id: kafkaui
    client-secret: ${OIDC_CLIENT_SECRET}
    scopes: [openid, profile, email]
    redirect-url: http://localhost:8080/api/v1/auth/callback
  session:
    secret: ${SESSION_SECRET}
    max-age: 86400
  rbac:
    - role: admin
      clusters: ["*"]
      actions: ["*"]
    - role: viewer
      clusters: [production]
      actions: [read]

clusters:
  - name: production
    bootstrap-servers: broker1:9092,broker2:9092
    sasl:
      mechanism: SCRAM-SHA-256
      username: admin
      password: ${KAFKA_PASSWORD}
    tls:
      enabled: true
      ca-file: /etc/ssl/certs/ca.pem
    schema-registry:
      url: http://schema-registry:8081
    kafka-connect:
      - name: connect-cluster
        url: http://kafka-connect:8083
    ksql:
      url: http://ksqldb:8088

  - name: staging
    bootstrap-servers: staging-kafka:9092

data-masking:
  rules:
    - topic-pattern: "payments.*"
      fields:
        - path: card_number
          type: mask
        - path: ssn
          type: hide
        - path: email
          type: hash
```

## API

Interactive Swagger UI is available at `/api/v1/docs`. The OpenAPI spec can be downloaded from `/api/v1/docs/openapi.yaml`.

All API routes are under `/api/v1/`. WebSocket endpoints are under `/ws/`.

## License

[GNU General Public License v2.0](LICENSE)
