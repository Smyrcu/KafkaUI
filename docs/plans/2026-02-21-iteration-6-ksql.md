# Iteration 6: KSQL -- Design

## Overview

Add KSQL query execution to KafkaUI. Users can write and execute KSQL statements through a SQL editor with quick action buttons, view results as JSON, and check KSQL server info. All requests are proxied through the KafkaUI backend to the KSQL REST API.

## Backend: KSQL Client

New package `internal/ksql/` with a stateless HTTP client wrapping the KSQL REST API.

### ksql.Client

```go
type Client struct {
    baseURL    string
    httpClient *http.Client
}

func New(baseURL string) *Client
func (c *Client) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error)
func (c *Client) Info(ctx context.Context) (*ServerInfo, error)
```

- `Execute` sends `POST /ksql` to the KSQL server with the query string, returns parsed response.
- `Info` sends `GET /info` to the KSQL server, returns server metadata.

Both methods propagate HTTP errors from the KSQL server as structured error responses.

## Backend API

### Execute KSQL Statement

`POST /api/v1/clusters/{clusterName}/ksql`

Request body:
```json
{
  "query": "SHOW STREAMS;"
}
```

Response:
```json
{
  "type": "streams",
  "statementText": "SHOW STREAMS;",
  "warnings": [],
  "data": [...]
}
```

Returns 400 if the cluster has no KSQL configured (`ClusterConfig.KSQL.URL` is empty). Returns 502 if the KSQL server is unreachable or returns an error.

### Get KSQL Server Info

`GET /api/v1/clusters/{clusterName}/ksql/info`

Response:
```json
{
  "version": "0.29.0",
  "kafkaClusterId": "abc123",
  "ksqlServiceId": "default_"
}
```

Returns 400 if the cluster has no KSQL configured. Returns 502 if the KSQL server is unreachable.

### Handler

Handler in `internal/api/handlers/ksql.go` with methods `Execute` and `Info`. The handler creates a `ksql.Client` from `ClusterConfig.KSQL.URL` on each request. If the URL is not configured, it returns 400 with a descriptive error message.

## Configuration

KSQL URL is added to the cluster config:

```yaml
clusters:
  - name: local
    brokers: ["localhost:9092"]
    ksql:
      url: "http://localhost:8088"
```

```go
type KSQLConfig struct {
    URL string `yaml:"url"`
}
```

Added as `KSQL KSQLConfig` field on `ClusterConfig`.

## Data Models

### Go (internal/ksql)

```go
type ExecuteRequest struct {
    Query string `json:"query"`
}

type ExecuteResponse struct {
    Type          string          `json:"type"`
    StatementText string         `json:"statementText"`
    Warnings      []Warning       `json:"warnings"`
    Data          json.RawMessage `json:"data"`
}

type Warning struct {
    Message string `json:"message"`
}

type ServerInfo struct {
    Version        string `json:"version"`
    KafkaClusterID string `json:"kafkaClusterId"`
    KSQLServiceID  string `json:"ksqlServiceId"`
}
```

### TypeScript (frontend/src/lib/api.ts)

```typescript
interface KsqlRequest { query: string; }
interface KsqlResponse {
  type: string; statementText: string; warnings: { message: string }[]; data: unknown;
}
interface KsqlServerInfo {
  version: string; kafkaClusterId: string; ksqlServiceId: string;
}
```

API functions:

```typescript
function executeKsql(clusterName: string, req: KsqlRequest): Promise<KsqlResponse>
function getKsqlInfo(clusterName: string): Promise<KsqlServerInfo>
```

## Frontend

### KsqlPage

Single page at `/clusters/:clusterName/ksql` with the following layout:

- **SQL Editor**: `<textarea>` for writing KSQL statements, with monospace font styling.
- **Quick Action Buttons**: Row of buttons that populate the textarea with common statements: `SHOW STREAMS`, `SHOW TABLES`, `SHOW TOPICS`, `SHOW QUERIES`.
- **Execute Button**: Sends the textarea content to the execute endpoint. Shows loading state during execution.
- **Server Info**: Small section (or button) displaying KSQL server version and service ID via the info endpoint.
- **Result Display**: JSON output rendered in a `<pre>` block with syntax highlighting. Shows the response type and statement text as a header. Warnings displayed as alert banners when present.
- **Error Handling**: API errors displayed inline. 400 errors (no KSQL configured) show a descriptive empty state suggesting the user configure KSQL for the cluster.

### Routes

```
/clusters/:clusterName/ksql -> KsqlPage
```

## KSQL REST API Reference (proxied)

The backend proxies to these KSQL server endpoints:

| KSQL Endpoint | Method | Used By |
|---------------|--------|---------|
| `/ksql` | POST | Execute statement |
| `/info` | GET | Server info |

The KSQL `/ksql` endpoint expects `{"ksql": "..."}` as the body field name. The backend translates from our `query` field to `ksql` when forwarding.

## Testing

### Client Unit Tests (internal/ksql/client_test.go) -- 7 tests

1. Execute with valid statement returns parsed response
2. Execute with invalid statement returns error
3. Execute propagates KSQL server error response
4. Execute handles network error
5. Info returns server metadata
6. Info handles network error
7. Info handles malformed response

### Handler Tests (internal/api/handlers/ksql_test.go) -- 6 tests

1. Execute returns result for valid query
2. Execute returns 400 when KSQL not configured
3. Execute returns 400 for empty query
4. Execute returns 502 when KSQL server unreachable
5. Info returns server info
6. Info returns 400 when KSQL not configured

## Dependencies

No new external dependencies. Uses Go standard library `net/http` for the KSQL HTTP client. Frontend uses existing shadcn/ui components (Button, Card, Textarea).

## File Changes

Backend:
- `internal/ksql/client.go` -- new, HTTP client wrapping KSQL REST API
- `internal/ksql/client_test.go` -- new, client unit tests with httptest server
- `internal/api/handlers/ksql.go` -- new, handler with Execute and Info methods
- `internal/api/handlers/ksql_test.go` -- new, handler unit tests
- `internal/config/config.go` -- add `KSQLConfig` struct and field on `ClusterConfig`
- `internal/api/router.go` -- register KSQL routes
- `internal/api/handlers/openapi.yaml` -- add KSQL endpoints and schemas

Frontend:
- `frontend/src/lib/api.ts` -- add KSQL types and API functions
- `frontend/src/pages/KsqlPage.tsx` -- new, SQL editor page
- `frontend/src/App.tsx` -- register KSQL route
