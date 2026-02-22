# Iteration 7: ACL Management -- Design

## Overview

Add ACL (Access Control List) management to KafkaUI. Users can list all ACLs with search and filtering, create new ACL entries via a dialog with dropdowns, and delete individual entries. Uses franz-go's `kmsg` package for direct Kafka protocol ACL operations (DescribeACLs, CreateACLs, DeleteACLs). ACLs are a Kafka-native feature -- no external service needed.

## Backend API

### List ACLs

`GET /api/v1/clusters/{clusterName}/acls`

Response: JSON array of ACL entries:
```json
[
  {
    "resourceType": "TOPIC",
    "resourceName": "orders",
    "patternType": "LITERAL",
    "principal": "User:alice",
    "host": "*",
    "operation": "READ",
    "permission": "ALLOW"
  }
]
```

Implementation: Build a `kmsg.DescribeACLsRequest` with permissive filters (resource type ANY, pattern type ANY) to fetch all ACLs. Convert each `kmsg.DescribeACLsResponseResource` into `ACLEntry` structs using conversion helpers for kmsg constants to string representations.

### Create ACL

`POST /api/v1/clusters/{clusterName}/acls`

Request body:
```json
{
  "resourceType": "TOPIC",
  "resourceName": "orders",
  "patternType": "LITERAL",
  "principal": "User:alice",
  "host": "*",
  "operation": "READ",
  "permission": "ALLOW"
}
```

Implementation: Convert the request fields from string representations to `kmsg` constants, build a `kmsg.CreateACLsRequest` with a single creation entry, and issue it against the cluster. Return an error if the response contains a non-zero error code.

### Delete ACL

`POST /api/v1/clusters/{clusterName}/acls/delete`

Request body (same shape as an ACL entry -- matches on all fields):
```json
{
  "resourceType": "TOPIC",
  "resourceName": "orders",
  "patternType": "LITERAL",
  "principal": "User:alice",
  "host": "*",
  "operation": "READ",
  "permission": "ALLOW"
}
```

Uses POST with a body instead of DELETE because matching an ACL requires multiple fields that do not fit cleanly into URL parameters.

Implementation: Convert request fields to `kmsg` constants, build a `kmsg.DeleteACLsRequest` with a single filter entry, and issue it. Return an error if the response contains a non-zero error code.

## Backend: Kafka Client Methods

New file `internal/kafka/acl.go` with three methods on the existing `kafka.Client`:

- **`ListACLs(ctx) ([]ACLEntry, error)`** -- Issues a `kmsg.DescribeACLsRequest` with resource type `kmsg.ACLResourceTypeAny` and pattern type `kmsg.ACLResourcePatternTypeAny`. Iterates response resources and their ACL entries, converting each to an `ACLEntry`. Results sorted by resource type, resource name, then principal.
- **`CreateACL(ctx, entry ACLEntry) error`** -- Converts the entry to a `kmsg.CreateACLsRequestCreation` and issues a `kmsg.CreateACLsRequest`. Checks the per-creation error code in the response.
- **`DeleteACL(ctx, entry ACLEntry) error`** -- Converts the entry to a `kmsg.DeleteACLsRequestFilter` and issues a `kmsg.DeleteACLsRequest`. Checks the per-filter error code in the response.

### Conversion Helpers

String-to-kmsg-constant and kmsg-constant-to-string helpers for:
- Resource types: `TOPIC`, `GROUP`, `CLUSTER`, `TRANSACTIONAL_ID` (mapped to `kmsg.ACLResourceTypeTopic`, etc.)
- Pattern types: `LITERAL`, `PREFIXED` (mapped to `kmsg.ACLResourcePatternTypeLiteral`, etc.)
- Operations: `ALL`, `READ`, `WRITE`, `CREATE`, `DELETE`, `ALTER`, `DESCRIBE`, `CLUSTER_ACTION`, `DESCRIBE_CONFIGS`, `ALTER_CONFIGS`, `IDEMPOTENT_WRITE` (mapped to `kmsg.ACLOperationAll`, etc.)
- Permissions: `ALLOW`, `DENY` (mapped to `kmsg.ACLPermissionTypeAllow`, etc.)

## Data Models

### Go (internal/kafka/acl.go)

```go
type ACLEntry struct {
    ResourceType string `json:"resourceType"`
    ResourceName string `json:"resourceName"`
    PatternType  string `json:"patternType"`
    Principal    string `json:"principal"`
    Host         string `json:"host"`
    Operation    string `json:"operation"`
    Permission   string `json:"permission"`
}
```

### TypeScript (frontend/src/lib/api.ts)

```typescript
interface ACLEntry {
  resourceType: string;
  resourceName: string;
  patternType: string;
  principal: string;
  host: string;
  operation: string;
  permission: string;
}
```

## ACL Field Values

| Field | Valid Values |
|-------|-------------|
| Resource Type | `TOPIC`, `GROUP`, `CLUSTER`, `TRANSACTIONAL_ID` |
| Pattern Type | `LITERAL`, `PREFIXED` |
| Operation | `ALL`, `READ`, `WRITE`, `CREATE`, `DELETE`, `ALTER`, `DESCRIBE`, `CLUSTER_ACTION`, `DESCRIBE_CONFIGS`, `ALTER_CONFIGS`, `IDEMPOTENT_WRITE` |
| Permission | `ALLOW`, `DENY` |

## Frontend

### AclPage

Single page handling list, create, and delete.

- **Search input** to filter ACL entries by any field (resource name, principal, etc.)
- **Filter dropdowns** for resource type, permission type
- **Table columns**: Resource Type, Resource Name, Pattern Type, Principal, Host, Operation, Permission, Actions (delete button)
- **Create ACL button** opens a dialog with:
  - Resource Type dropdown (TOPIC, GROUP, CLUSTER, TRANSACTIONAL_ID)
  - Resource Name text input
  - Pattern Type dropdown (LITERAL, PREFIXED)
  - Principal text input (e.g., `User:alice`)
  - Host text input (default `*`)
  - Operation dropdown (ALL, READ, WRITE, etc.)
  - Permission dropdown (ALLOW, DENY)
  - Submit button with loading state and error display
- **Delete button** per row with confirmation dialog
- Empty state when no ACLs exist or match filters

### Routes

```
/clusters/:clusterName/acls -> AclPage
```

## Dependencies

No new dependencies. Uses existing franz-go `kmsg` package (already a transitive dependency of franz-go) for raw Kafka protocol requests and existing shadcn/ui components (Badge, Card, Table, Dialog, Input, Button, Label, Select).

## Testing

### Handler Tests (internal/api/handlers/acl_test.go)

10 tests covering error paths and request validation:

1. `TestListACLs_Success` -- mock client returns entries, verify JSON response
2. `TestListACLs_ClusterNotFound` -- unknown cluster name returns 404
3. `TestListACLs_KafkaError` -- client error returns 500
4. `TestCreateACL_Success` -- valid request returns 201
5. `TestCreateACL_InvalidBody` -- malformed JSON returns 400
6. `TestCreateACL_MissingFields` -- missing required fields returns 400
7. `TestCreateACL_InvalidFieldValue` -- invalid resource type / operation / etc. returns 400
8. `TestCreateACL_ClusterNotFound` -- unknown cluster returns 404
9. `TestDeleteACL_Success` -- valid request returns 204
10. `TestDeleteACL_ClusterNotFound` -- unknown cluster returns 404

No client unit tests -- ACL protocol operations require a real Kafka broker with authorization enabled, so these are not suitable for unit testing with mocks.

## File Changes

Backend:
- `internal/kafka/acl.go` -- new file: `ACLEntry` type, conversion helpers, `ListACLs`, `CreateACL`, `DeleteACL` methods on `Client`
- `internal/api/handlers/acl.go` -- new handler with `List`, `Create`, `Delete`
- `internal/api/handlers/acl_test.go` -- handler unit tests (10 tests)
- `internal/api/router.go` -- register ACL routes
- `internal/api/handlers/openapi.yaml` -- add ACL endpoints and schemas

Frontend:
- `frontend/src/lib/api.ts` -- add `ACLEntry` type and API functions (`fetchACLs`, `createACL`, `deleteACL`)
- `frontend/src/pages/AclPage.tsx` -- ACL list with search, filter, create dialog, delete per entry
- `frontend/src/App.tsx` -- register ACL route
