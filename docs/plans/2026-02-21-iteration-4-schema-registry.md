# Iteration 4: Schema Registry -- Design

## Overview

Add Schema Registry management to KafkaUI. Users can list all subjects with search, view subject details with all schema versions, register new schema versions, and delete subjects. All requests are proxied through the KafkaUI backend to the Confluent Schema Registry REST API.

## Backend API

### List Subjects

`GET /api/v1/clusters/{clusterName}/schemas`

Response: JSON array of subject summaries with latest version info:
```json
[
  {
    "subject": "orders-value",
    "latestVersion": 3,
    "latestSchemaId": 42,
    "schemaType": "AVRO"
  }
]
```

Implementation: Call `GET /subjects` on the Schema Registry to get all subject names, then `GET /subjects/{subject}/versions/latest` for each to get latest version, schema ID, and type. Returns 400 if the cluster has no Schema Registry configured.

### Register Schema

`POST /api/v1/clusters/{clusterName}/schemas`

Request body:
```json
{
  "subject": "orders-value",
  "schema": "{\"type\":\"record\",\"name\":\"Order\",...}",
  "schemaType": "AVRO"
}
```

Response:
```json
{
  "id": 43
}
```

Implementation: Proxy to `POST /subjects/{subject}/versions` on the Schema Registry. `schemaType` defaults to `"AVRO"` if omitted. Also accepts `"JSON"` and `"PROTOBUF"`.

### Subject Details

`GET /api/v1/clusters/{clusterName}/schemas/{subject}`

Response: Full subject detail with compatibility and all versions:
```json
{
  "subject": "orders-value",
  "compatibility": "BACKWARD",
  "versions": [
    {
      "version": 1,
      "id": 10,
      "schema": "{\"type\":\"record\",...}",
      "schemaType": "AVRO"
    },
    {
      "version": 2,
      "id": 25,
      "schema": "{\"type\":\"record\",...}",
      "schemaType": "AVRO"
    }
  ]
}
```

Implementation: `GET /subjects/{subject}/versions` to list version numbers, then `GET /subjects/{subject}/versions/{version}` for each version's schema. `GET /config/{subject}` for subject-level compatibility, falling back to `GET /config` for global default.

### Delete Subject

`DELETE /api/v1/clusters/{clusterName}/schemas/{subject}`

Response: 204 No Content.

Implementation: Proxy to `DELETE /subjects/{subject}` on the Schema Registry.

## Backend: Schema Registry Client

New package `internal/schema/` with a stateless HTTP client wrapping the Confluent Schema Registry REST API.

`schema.Client` holds a base URL and `*http.Client`. It is created on-demand per request from `ClusterConfig.SchemaRegistry.URL` -- no persistent state or connection pooling beyond the standard `net/http` client.

Methods on `schema.Client`:

- **`ListSubjects(ctx) ([]SubjectInfo, error)`** -- Calls `GET /subjects`, then `GET /subjects/{subject}/versions/latest` for each subject. Returns subject name, latest version, latest schema ID, and schema type.
- **`GetSubjectDetails(ctx, subject) (*SchemaDetail, error)`** -- Calls `GET /subjects/{subject}/versions` to list versions, fetches each version's schema, and calls `GET /config/{subject}` (fallback `GET /config`) for compatibility level.
- **`RegisterSchema(ctx, req) (*CreateSchemaResponse, error)`** -- Calls `POST /subjects/{subject}/versions` with the schema body.
- **`DeleteSubject(ctx, subject) error`** -- Calls `DELETE /subjects/{subject}`.

## Data Models

### Go (internal/schema/client.go)

```go
type SubjectInfo struct {
    Subject        string `json:"subject"`
    LatestVersion  int    `json:"latestVersion"`
    LatestSchemaID int    `json:"latestSchemaId"`
    SchemaType     string `json:"schemaType"`
}

type SchemaDetail struct {
    Subject       string          `json:"subject"`
    Compatibility string          `json:"compatibility"`
    Versions      []SchemaVersion `json:"versions"`
}

type SchemaVersion struct {
    Version    int    `json:"version"`
    ID         int    `json:"id"`
    Schema     string `json:"schema"`
    SchemaType string `json:"schemaType"`
}

type CreateSchemaRequest struct {
    Subject    string `json:"subject"`
    Schema     string `json:"schema"`
    SchemaType string `json:"schemaType"`
}

type CreateSchemaResponse struct {
    ID int `json:"id"`
}
```

### TypeScript (frontend/src/lib/api.ts)

```typescript
interface SchemaSubjectInfo {
  subject: string; latestVersion: number; latestSchemaId: number; schemaType: string;
}
interface SchemaDetail {
  subject: string; compatibility: string; versions: SchemaVersion[];
}
interface SchemaVersion {
  version: number; id: number; schema: string; schemaType: string;
}
interface CreateSchemaRequest {
  subject: string; schema: string; schemaType: string;
}
```

## Frontend

### SchemaRegistryPage

- Search input to filter subjects by name
- Table columns: Subject (link to detail), Latest Version, Schema ID, Type (badge)
- "Register Schema" button opens a create dialog
- **Create dialog**: Subject input, schema type selector (AVRO / JSON / PROTOBUF), schema text area, submit button with loading state and error display
- Empty state when no subjects match or no Schema Registry is configured

### SchemaDetailPage

- Header with subject name and compatibility badge
- Delete button with confirmation dialog
- **Versions card**: table listing all versions sorted descending (newest first). Columns: Version, Schema ID, Type.
- Clicking a version row expands to show the full schema with JSON formatting and syntax highlighting
- Latest version schema is expanded by default

### Routes

```
/clusters/:clusterName/schema-registry             -> SchemaRegistryPage
/clusters/:clusterName/schema-registry/:subject     -> SchemaDetailPage
```

## Configuration

Schema Registry URL is added to the existing cluster config:

```yaml
clusters:
  - name: local
    brokers: ["localhost:9092"]
    schemaRegistry:
      url: "http://localhost:8081"
```

No authentication for the Schema Registry in this iteration. If `schemaRegistry.url` is not set, schema endpoints return 400 with a descriptive error message.

## Testing

### Backend
- Schema client unit tests with `httptest` mock server (8 tests): list subjects, get details, register schema, delete subject, error handling for each
- Handler tests for error paths (10 tests): cluster not found, no Schema Registry configured, invalid request body, subject not found, Schema Registry HTTP errors

### Frontend
- `SchemaRegistryPage` component tests: renders subject list, search filtering, create dialog interaction
- `SchemaDetailPage` component tests: renders versions, schema viewer, delete confirmation

## Dependencies

No new external dependencies. Uses `net/http` for the Schema Registry client and existing shadcn/ui components (Badge, Card, Table, Dialog, Input, Button, Label, Textarea).

## File Changes

Backend:
- `internal/schema/client.go` -- new: Schema Registry HTTP client with `ListSubjects`, `GetSubjectDetails`, `RegisterSchema`, `DeleteSubject`
- `internal/schema/client_test.go` -- new: unit tests with httptest mock server
- `internal/api/handlers/schema.go` -- new: handler with `ListSubjects`, `GetSubjectDetails`, `RegisterSchema`, `DeleteSubject`
- `internal/api/handlers/schema_test.go` -- new: handler unit tests
- `internal/api/router.go` -- register schema routes
- `internal/api/handlers/openapi.yaml` -- add schema endpoints and schemas
- `internal/config/config.go` -- add `SchemaRegistry` field to `ClusterConfig`

Frontend:
- `frontend/src/lib/api.ts` -- add schema types and API functions
- `frontend/src/pages/SchemaRegistryPage.tsx` -- subject list with search and create dialog
- `frontend/src/pages/SchemaDetailPage.tsx` -- subject detail with version list and schema viewer
- `frontend/src/App.tsx` -- replace placeholder with real schema registry routes
