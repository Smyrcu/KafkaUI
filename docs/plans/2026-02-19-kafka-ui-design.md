# KafkaUI — Design Document

## Overview

A modern, open-source web UI for managing and monitoring Apache Kafka clusters. Inspired by provectus/kafka-ui with full feature parity as the goal.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go (1.23+) |
| Frontend | React + shadcn/ui + Tailwind CSS + Vite |
| Kafka Client | franz-go |
| HTTP Router | chi |
| WebSocket | gorilla/websocket |
| Auth | coreos/go-oidc + golang.org/x/oauth2 |
| Config | gopkg.in/yaml.v3 |
| Logging | slog (stdlib) |
| Deployment | Single binary (go:embed) + Docker |

## Architecture

Monorepo with embedded frontend. Go binary serves both the API and static frontend files via `go:embed`.

```
KafkaUI/
├── cmd/kafkaui/              # main.go entry point
├── internal/
│   ├── kafka/                # Kafka admin client (franz-go)
│   ├── api/                  # REST + WebSocket handlers (chi)
│   │   ├── handlers/         # HTTP handlers per domain
│   │   ├── middleware/       # Auth, logging, CORS
│   │   └── ws/               # WebSocket handlers
│   ├── auth/                 # OAuth2/OIDC + RBAC
│   ├── config/               # YAML config parsing
│   ├── schema/               # Schema Registry HTTP client
│   ├── connect/              # Kafka Connect HTTP client
│   ├── ksql/                 # KSQL HTTP client
│   └── masking/              # Data masking engine
├── frontend/                 # React app
│   ├── src/
│   │   ├── components/
│   │   │   ├── layout/       # Sidebar, TopBar, Breadcrumbs
│   │   │   ├── clusters/     # ClusterList, ClusterOverview
│   │   │   ├── brokers/      # BrokerList, BrokerDetails
│   │   │   ├── topics/       # TopicList, TopicDetails, TopicMessages, TopicCreate
│   │   │   ├── consumers/    # ConsumerGroupList, ConsumerGroupDetails
│   │   │   ├── schemas/      # SchemaList, SchemaDetails, SchemaCreate
│   │   │   ├── connect/      # ConnectorList, ConnectorDetails, ConnectorCreate
│   │   │   ├── ksql/         # KsqlEditor
│   │   │   ├── acl/          # AclList, AclCreate
│   │   │   └── shared/       # DataTable, JsonViewer, SearchBar, StatusBadge
│   │   ├── hooks/            # useWebSocket, useKafkaApi, useAuth
│   │   ├── lib/              # API client, WS client, auth helpers
│   │   ├── pages/            # React Router pages
│   │   └── App.tsx
│   └── dist/                 # Build output (embedded in Go binary)
├── config.example.yaml
├── Dockerfile
├── Makefile
└── README.md
```

## API Design

### REST API (`/api/v1/`)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /clusters | List configured clusters |
| GET | /clusters/:id/brokers | List brokers |
| GET | /clusters/:id/topics | List topics |
| POST | /clusters/:id/topics | Create topic |
| DELETE | /clusters/:id/topics/:name | Delete topic |
| GET | /clusters/:id/topics/:name | Topic details (config, partitions) |
| GET | /clusters/:id/topics/:name/messages | Browse messages (query params: offset, timestamp, limit, key, value filter) |
| POST | /clusters/:id/topics/:name/messages | Produce message |
| GET | /clusters/:id/consumer-groups | List consumer groups |
| GET | /clusters/:id/consumer-groups/:name | CG details with lag |
| POST | /clusters/:id/consumer-groups/:name/reset | Reset offsets |
| GET | /clusters/:id/schemas | List schemas (Schema Registry) |
| POST | /clusters/:id/schemas | Create schema |
| GET | /clusters/:id/schemas/:subject | Schema details + versions |
| DELETE | /clusters/:id/schemas/:subject | Delete schema |
| GET | /clusters/:id/connectors | List connectors (Kafka Connect) |
| POST | /clusters/:id/connectors | Create connector |
| GET | /clusters/:id/connectors/:name | Connector details |
| PUT | /clusters/:id/connectors/:name | Update connector config |
| DELETE | /clusters/:id/connectors/:name | Delete connector |
| POST | /clusters/:id/connectors/:name/restart | Restart connector |
| POST | /clusters/:id/connectors/:name/pause | Pause connector |
| POST | /clusters/:id/connectors/:name/resume | Resume connector |
| POST | /clusters/:id/ksql | Execute KSQL query |
| GET | /clusters/:id/acls | List ACLs |
| POST | /clusters/:id/acls | Create ACL |
| DELETE | /clusters/:id/acls | Delete ACL |

### WebSocket (`/ws/`)

| Endpoint | Description |
|----------|-------------|
| /ws/clusters/:id/topics/:name/live | Live tail messages from topic |
| /ws/clusters/:id/metrics | Streaming cluster metrics |

## Configuration

```yaml
server:
  port: 8080
  base-path: ""

auth:
  enabled: true
  type: oidc
  oidc:
    issuer: https://keycloak.example.com/realms/kafka
    client-id: kafka-ui
    client-secret: ${OIDC_CLIENT_SECRET}
    scopes: [openid, profile, email]
  rbac:
    - role: admin
      clusters: ["*"]
      actions: ["*"]
    - role: viewer
      clusters: ["production"]
      actions: [view_topics, view_messages, view_consumer_groups]

clusters:
  - name: production
    bootstrap-servers: kafka-prod-1:9092,kafka-prod-2:9092
    tls:
      enabled: true
      ca-file: /certs/ca.pem
    sasl:
      mechanism: SCRAM-SHA-512
      username: admin
      password: ${KAFKA_PROD_PASSWORD}
    schema-registry:
      url: https://schema-registry-prod:8081
    kafka-connect:
      - name: prod-connect
        url: http://connect-prod:8083
    ksql:
      url: http://ksql-prod:8088

  - name: staging
    bootstrap-servers: kafka-staging:9092

data-masking:
  rules:
    - topic-pattern: "*.sensitive.*"
      fields:
        - path: "$.email"
          type: mask
        - path: "$.ssn"
          type: hide
```

Environment variable expansion supported via `${VAR_NAME}` syntax.

## Data Masking

- Rules defined per topic pattern (glob matching)
- JSONPath selectors for field targeting
- Masking types: `mask` (partial: `j***@e***.com`), `hide` (full: `****`), `hash` (SHA256)
- Applied server-side before sending to frontend
- Configured in YAML, not editable from UI

## Authentication & Authorization

### Auth Flow

1. User navigates to UI → redirect to OIDC provider
2. User authenticates → callback with authorization code
3. Backend exchanges code for tokens, creates session (HTTP-only secure cookie)
4. RBAC checked per request based on token roles/claims

### RBAC Actions

- `view_topics`, `create_topics`, `delete_topics`
- `view_messages`, `produce_messages`
- `view_consumer_groups`, `reset_offsets`
- `view_schemas`, `manage_schemas`
- `view_connectors`, `manage_connectors`
- `execute_ksql`
- `view_acls`, `manage_acls`

## Frontend Views

1. **Dashboard** — Overview of all clusters: broker count, topic count, partition count, throughput metrics
2. **Topics** — Filterable/sortable table; click → details (config, partitions, consumer groups, message browser)
3. **Messages Browser** — Filter by offset/timestamp/key/value, live tail via WebSocket, JSON/Avro/Protobuf viewer
4. **Consumer Groups** — List with per-partition lag, reset offsets functionality
5. **Schema Registry** — Schema list, version history, compatibility settings, create/edit editor
6. **Kafka Connect** — Connector list with status badges, CRUD with JSON config editor
7. **KSQL** — SQL editor with syntax highlighting, tabular results display
8. **ACL** — ACL list with filtering, create/delete operations
9. **Settings** — Cluster info, auth status (read-only)

## Deployment

### Single Binary

```bash
# Build
make build
# Run
./kafkaui --config config.yaml
```

### Docker

```dockerfile
FROM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/ .
RUN npm ci && npm run build

FROM golang:1.23-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 go build -o kafkaui ./cmd/kafkaui

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=backend /app/kafkaui /usr/local/bin/
EXPOSE 8080
ENTRYPOINT ["kafkaui"]
```

### Makefile

- `make dev` — hot reload backend (air) + frontend (vite) concurrently
- `make build` — build frontend + Go binary
- `make docker` — build Docker image
- `make test` — Go tests + frontend tests

## Implementation Iterations

| # | Scope | Description |
|---|-------|-------------|
| 1 | **Core scaffold** | Project structure, config YAML, cluster/broker/topic list, topic CRUD |
| 2 | **Messages** | Message browser (offset/timestamp/filters), live tail (WS), produce messages |
| 3 | **Consumer Groups** | CG list, lag monitoring, reset offsets |
| 4 | **Schema Registry** | Schema CRUD, versions, compatibility checks |
| 5 | **Auth** | OAuth2/OIDC login flow, session management, RBAC middleware |
| 6 | **Kafka Connect** | Connector list, status, CRUD, JSON config editor |
| 7 | **KSQL** | SQL editor, query execution, tabular results |
| 8 | **ACL** | ACL list, create/delete operations |
| 9 | **Data Masking** | Field-level masking rules, JSONPath, server-side filtering |
| 10 | **Polish** | Multi-cluster dashboard, metrics, dark mode, responsive layout |
