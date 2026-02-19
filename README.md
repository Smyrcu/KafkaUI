# KafkaUI

Web UI for managing Apache Kafka clusters. Single Go binary with embedded React frontend.

## Quick Start (Docker)

```bash
docker compose up --build
```

Open http://localhost:8080 — Kafka and KafkaUI start together, preconfigured.

## Development

### Prerequisites

- Go 1.25+
- Node.js 22+
- A running Kafka instance (or use `docker compose up kafka` to start one)

### Setup

```bash
# Install frontend dependencies
cd frontend && npm install && cd ..

# Copy example config
cp config.example.yaml config.yaml
```

Edit `config.yaml` to point to your Kafka broker(s).

### Run

```bash
# Start both backend and frontend dev servers
make dev
```

- Frontend (Vite HMR): http://localhost:5173
- Backend API: http://localhost:8080
- Swagger UI: http://localhost:8080/api/v1/docs

### Build

```bash
# Build single binary with embedded frontend
make build

# Run it
./kafkaui --config config.yaml
```

### Docker

```bash
# Build image
make docker

# Run with custom config
docker run -p 8080:8080 -v $(pwd)/config.yaml:/etc/kafkaui/config.yaml kafkaui
```

### Tests

```bash
make test
```

## Configuration

```yaml
server:
  port: 8080

clusters:
  - name: production
    bootstrap-servers: broker1:9092,broker2:9092
    sasl:
      mechanism: SCRAM-SHA-256
      username: admin
      password: ${KAFKA_PASSWORD}
    tls:
      enabled: true
```

Environment variables can be used with `${VAR_NAME}` syntax.

## Tech Stack

- **Backend:** Go, chi, franz-go
- **Frontend:** React 19, Vite, Tailwind CSS v4, shadcn/ui, TanStack Query
- **Deployment:** Single binary with `go:embed`, Docker
