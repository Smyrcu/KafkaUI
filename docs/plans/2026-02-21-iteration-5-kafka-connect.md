# Iteration 5: Kafka Connect -- Design

## Overview

Add Kafka Connect management to KafkaUI. Users can list all connectors with search, view connector status and configuration, create new connectors, update configuration, delete connectors, and manage lifecycle (restart/pause/resume). All requests are proxied through the KafkaUI backend to the Kafka Connect REST API. Supports multiple Kafka Connect clusters per Kafka cluster.

## Backend API

### List Connectors

`GET /api/v1/clusters/{clusterName}/connectors`

Response: JSON array of connector summaries aggregated from all configured connect clusters:
```json
[
  {
    "name": "jdbc-source-orders",
    "type": "source",
    "state": "RUNNING",
    "workerId": "connect-1:8083",
    "connectCluster": "production-connect"
  }
]
```

Implementation: For each configured Kafka Connect cluster, call `GET /connectors?expand=info&expand=status` and merge the results. Each entry is annotated with the `connectCluster` name it came from. Returns 400 if the cluster has no Kafka Connect configured.

### Create Connector

`POST /api/v1/clusters/{clusterName}/connectors`

Request body:
```json
{
  "name": "jdbc-source-orders",
  "config": {
    "connector.class": "io.confluent.connect.jdbc.JdbcSourceConnector",
    "connection.url": "jdbc:postgresql://db:5432/orders",
    "topic.prefix": "orders-"
  },
  "connectCluster": "production-connect"
}
```

Response:
```json
{
  "name": "jdbc-source-orders",
  "type": "source",
  "state": "RUNNING",
  "workerId": "connect-1:8083",
  "config": { "..." : "..." },
  "tasks": [],
  "connectCluster": "production-connect"
}
```

Implementation: Proxy to `POST /connectors` on the specified connect cluster. `connectCluster` identifies which Kafka Connect cluster to target; if omitted and only one connect cluster is configured, it defaults to that one. Returns 400 if the specified connect cluster is not found.

### Connector Details

`GET /api/v1/clusters/{clusterName}/connectors/{connectorName}`

Response: Full connector detail with config, status, and tasks:
```json
{
  "name": "jdbc-source-orders",
  "type": "source",
  "state": "RUNNING",
  "workerId": "connect-1:8083",
  "config": {
    "connector.class": "io.confluent.connect.jdbc.JdbcSourceConnector",
    "connection.url": "jdbc:postgresql://db:5432/orders",
    "topic.prefix": "orders-"
  },
  "tasks": [
    {
      "id": 0,
      "state": "RUNNING",
      "workerId": "connect-1:8083",
      "trace": ""
    }
  ],
  "connectCluster": "production-connect"
}
```

Implementation: Search across all configured connect clusters for the connector. Call `GET /connectors/{name}` for config and `GET /connectors/{name}/status` for status and tasks. Returns 404 if the connector is not found in any connect cluster.

### Update Connector Config

`PUT /api/v1/clusters/{clusterName}/connectors/{connectorName}`

Request body:
```json
{
  "config": {
    "connector.class": "io.confluent.connect.jdbc.JdbcSourceConnector",
    "connection.url": "jdbc:postgresql://db:5432/orders",
    "topic.prefix": "orders-",
    "poll.interval.ms": "5000"
  }
}
```

Response: 200 with updated connector detail (same shape as GET detail).

Implementation: Locate the connector across connect clusters, then proxy to `PUT /connectors/{name}/config`.

### Delete Connector

`DELETE /api/v1/clusters/{clusterName}/connectors/{connectorName}`

Response: 204 No Content.

Implementation: Locate the connector across connect clusters, then proxy to `DELETE /connectors/{name}`.

### Restart Connector

`POST /api/v1/clusters/{clusterName}/connectors/{connectorName}/restart`

Response: 204 No Content.

Implementation: Locate the connector, then proxy to `POST /connectors/{name}/restart`.

### Pause Connector

`POST /api/v1/clusters/{clusterName}/connectors/{connectorName}/pause`

Response: 204 No Content.

Implementation: Locate the connector, then proxy to `PUT /connectors/{name}/pause`.

### Resume Connector

`POST /api/v1/clusters/{clusterName}/connectors/{connectorName}/resume`

Response: 204 No Content.

Implementation: Locate the connector, then proxy to `PUT /connectors/{name}/resume`.

## Backend: Kafka Connect Client

New package `internal/connect/` with a stateless HTTP client wrapping the Kafka Connect REST API.

`connect.Client` holds a name, base URL, and `*http.Client`. It is created on-demand per request from `ClusterConfig.KafkaConnect[].URL` -- no persistent state or connection pooling beyond the standard `net/http` client.

Methods on `connect.Client`:

- **`ListConnectors(ctx) ([]ConnectorInfo, error)`** -- Calls `GET /connectors?expand=info&expand=status`. Returns connector name, type, state, and worker ID for each.
- **`GetConnector(ctx, name) (*ConnectorDetail, error)`** -- Calls `GET /connectors/{name}` for config and `GET /connectors/{name}/status` for status and tasks.
- **`CreateConnector(ctx, req) (*ConnectorDetail, error)`** -- Calls `POST /connectors` with name and config.
- **`UpdateConnectorConfig(ctx, name, config) (*ConnectorDetail, error)`** -- Calls `PUT /connectors/{name}/config` with the config map.
- **`DeleteConnector(ctx, name) error`** -- Calls `DELETE /connectors/{name}`.
- **`RestartConnector(ctx, name) error`** -- Calls `POST /connectors/{name}/restart`.
- **`PauseConnector(ctx, name) error`** -- Calls `PUT /connectors/{name}/pause`.
- **`ResumeConnector(ctx, name) error`** -- Calls `PUT /connectors/{name}/resume`.

## Data Models

### Go (internal/connect/client.go)

```go
type ConnectorInfo struct {
    Name           string `json:"name"`
    Type           string `json:"type"`
    State          string `json:"state"`
    WorkerID       string `json:"workerId"`
    ConnectCluster string `json:"connectCluster"`
}

type ConnectorDetail struct {
    Name           string            `json:"name"`
    Type           string            `json:"type"`
    State          string            `json:"state"`
    WorkerID       string            `json:"workerId"`
    Config         map[string]string `json:"config"`
    Tasks          []TaskStatus      `json:"tasks"`
    ConnectCluster string            `json:"connectCluster"`
}

type TaskStatus struct {
    ID       int    `json:"id"`
    State    string `json:"state"`
    WorkerID string `json:"workerId"`
    Trace    string `json:"trace"`
}

type CreateConnectorRequest struct {
    Name           string            `json:"name"`
    Config         map[string]string `json:"config"`
    ConnectCluster string            `json:"connectCluster,omitempty"`
}
```

### TypeScript (frontend/src/lib/api.ts)

```typescript
interface ConnectorInfo {
  name: string; type: string; state: string; workerId: string; connectCluster: string;
}
interface ConnectorDetail {
  name: string; type: string; state: string; workerId: string;
  config: Record<string, string>; tasks: TaskStatus[]; connectCluster: string;
}
interface TaskStatus {
  id: number; state: string; workerId: string; trace: string;
}
interface CreateConnectorRequest {
  name: string; config: Record<string, string>; connectCluster?: string;
}
```

## Frontend

### KafkaConnectPage

- Search input to filter connectors by name
- Table columns: Name (link to detail), Type (badge: source/sink), State (badge), Worker ID, Connect Cluster
- State badges: RUNNING (default), PAUSED (secondary), FAILED (destructive), UNASSIGNED (outline)
- "Create Connector" button opens a create dialog
- **Create dialog**: Name input, connect cluster selector (dropdown if multiple configured), JSON config editor (textarea), submit button with loading state and error display
- Action buttons per row: Restart, Pause/Resume (toggles based on state), Delete (with confirmation)
- Empty state when no connectors match or no Kafka Connect is configured

### ConnectorDetailPage

- Header with connector name, type badge, and state badge
- Action buttons: Restart, Pause/Resume, Delete (with confirmation dialog)
- **Summary card**: Connector name, type, state, worker ID, connect cluster
- **Tasks card**: Table listing all tasks. Columns: Task ID, State (badge), Worker ID. Failed tasks show error trace in expandable row.
- **Config card**: Key-value table showing all connector configuration. "Edit Config" button opens a JSON editor dialog to update the full config map.

### Routes

```
/clusters/:clusterName/kafka-connect                    -> KafkaConnectPage
/clusters/:clusterName/kafka-connect/:connectorName      -> ConnectorDetailPage
```

## Configuration

Kafka Connect clusters are added to the existing cluster config:

```yaml
clusters:
  - name: local
    brokers: ["localhost:9092"]
    kafkaConnect:
      - name: "local-connect"
        url: "http://localhost:8083"
```

Multiple connect clusters can be configured per Kafka cluster. No authentication for the Kafka Connect REST API in this iteration. If `kafkaConnect` is not set or empty, connect endpoints return 400 with a descriptive error message.

## Testing

### Backend
- Connect client unit tests with `httptest` mock server (10 tests): list connectors, get details, create connector, update config, delete connector, restart/pause/resume, error handling for HTTP failures
- Handler tests for error paths (12 tests): cluster not found, no Kafka Connect configured, invalid request body, connector not found, connect cluster not found, Kafka Connect HTTP errors, lifecycle action errors

### Frontend
- `KafkaConnectPage` component tests: renders connector list, search filtering, create dialog interaction, action button clicks
- `ConnectorDetailPage` component tests: renders summary and tasks, config editor, lifecycle actions, delete confirmation

## Dependencies

No new external dependencies. Uses `net/http` for the Kafka Connect client and existing shadcn/ui components (Badge, Card, Table, Dialog, Input, Button, Label, Textarea, Select, DropdownMenu).

## File Changes

Backend:
- `internal/connect/client.go` -- new: Kafka Connect HTTP client with `ListConnectors`, `GetConnector`, `CreateConnector`, `UpdateConnectorConfig`, `DeleteConnector`, `RestartConnector`, `PauseConnector`, `ResumeConnector`
- `internal/connect/client_test.go` -- new: unit tests with httptest mock server
- `internal/api/handlers/connect.go` -- new: handler with list, create, get, update, delete, restart, pause, resume
- `internal/api/handlers/connect_test.go` -- new: handler unit tests
- `internal/api/router.go` -- register connect routes
- `internal/api/handlers/openapi.yaml` -- add connect endpoints and schemas
- `internal/config/config.go` -- add `KafkaConnect` field to `ClusterConfig`

Frontend:
- `frontend/src/lib/api.ts` -- add connect types and API functions
- `frontend/src/pages/KafkaConnectPage.tsx` -- connector list with search, create dialog, and action buttons
- `frontend/src/pages/ConnectorDetailPage.tsx` -- connector detail with tasks, config editor, and lifecycle actions
- `frontend/src/App.tsx` -- replace placeholder with real kafka connect routes
