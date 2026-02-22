# Iteration 3: Consumer Groups -- Design

## Overview

Add consumer group management to KafkaUI. Users can list all consumer groups with search and state filtering, view detailed group information including members and per-partition lag, and reset committed offsets for a group's topic.

## Backend API

### List Consumer Groups

`GET /api/v1/clusters/{clusterName}/consumer-groups`

Response: JSON array of consumer group summaries:
```json
[
  {
    "name": "my-service-group",
    "state": "Stable",
    "members": 3,
    "topics": 2,
    "coordinatorId": 1
  }
]
```

Implementation: Use `kadm.ListGroups` to discover all groups, then `kadm.DescribeGroups` to get state, member count, and assigned topics. Results are sorted alphabetically by name.

### Consumer Group Details

`GET /api/v1/clusters/{clusterName}/consumer-groups/{groupName}`

Response: Full group detail with members and per-topic/partition offsets:
```json
{
  "name": "my-service-group",
  "state": "Stable",
  "coordinatorId": 1,
  "members": [
    {
      "id": "my-service-abc123",
      "clientId": "my-service",
      "host": "/10.0.0.5",
      "topics": ["orders", "payments"]
    }
  ],
  "offsets": [
    {
      "topic": "orders",
      "totalLag": 150,
      "partitions": [
        { "partition": 0, "currentOffset": 1000, "endOffset": 1100, "lag": 100 },
        { "partition": 1, "currentOffset": 2000, "endOffset": 2050, "lag": 50 }
      ]
    }
  ]
}
```

Implementation: `kadm.DescribeGroups` for group metadata and members, `kadm.FetchOffsets` for committed offsets, `kadm.ListEndOffsets` for log-end offsets. Lag = endOffset - currentOffset (clamped to 0). Partitions sorted by ID, topics sorted alphabetically.

### Reset Consumer Group Offsets

`POST /api/v1/clusters/{clusterName}/consumer-groups/{groupName}/reset`

Request body:
```json
{
  "topic": "orders",
  "resetTo": "earliest"
}
```

`resetTo` must be `"earliest"` or `"latest"`. The group should be in `Empty` state (no active members) for the reset to succeed.

Implementation: `kadm.ListStartOffsets` or `kadm.ListEndOffsets` to resolve target offsets, then `kadm.CommitOffsets` to write them for the group.

## Backend: Kafka Client Methods

Three new methods on `kafka.Client`:

- **`ConsumerGroups(ctx) ([]ConsumerGroupInfo, error)`** -- Lists all groups via `ListGroups` + `DescribeGroups`. Returns name, state, member count, topic count, and coordinator ID.
- **`ConsumerGroupDetails(ctx, name) (*ConsumerGroupDetail, error)`** -- Describes a single group. Fetches committed offsets and end offsets to compute per-partition lag.
- **`ResetConsumerGroupOffsets(ctx, group, req) error`** -- Resolves start/end offsets for the requested topic, then commits them as the group's offsets.

## Data Models

### Go (internal/kafka/client.go)

```go
type ConsumerGroupInfo struct {
    Name          string `json:"name"`
    State         string `json:"state"`
    Members       int    `json:"members"`
    TopicCount    int    `json:"topics"`
    CoordinatorID int32  `json:"coordinatorId"`
}

type ConsumerGroupDetail struct {
    Name          string                     `json:"name"`
    State         string                     `json:"state"`
    CoordinatorID int32                      `json:"coordinatorId"`
    Members       []ConsumerGroupMember      `json:"members"`
    Offsets       []ConsumerGroupTopicOffset `json:"offsets"`
}

type ConsumerGroupMember struct {
    ID       string   `json:"id"`
    ClientID string   `json:"clientId"`
    Host     string   `json:"host"`
    Topics   []string `json:"topics"`
}

type ConsumerGroupTopicOffset struct {
    Topic      string                         `json:"topic"`
    Partitions []ConsumerGroupPartitionOffset `json:"partitions"`
    TotalLag   int64                          `json:"totalLag"`
}

type ConsumerGroupPartitionOffset struct {
    Partition     int32 `json:"partition"`
    CurrentOffset int64 `json:"currentOffset"`
    EndOffset     int64 `json:"endOffset"`
    Lag           int64 `json:"lag"`
}

type ResetOffsetsRequest struct {
    Topic   string `json:"topic"`
    ResetTo string `json:"resetTo"`
}
```

### TypeScript (frontend/src/lib/api.ts)

```typescript
interface ConsumerGroupInfo {
  name: string; state: string; members: number; topics: number; coordinatorId: number;
}
interface ConsumerGroupDetail {
  name: string; state: string; coordinatorId: number;
  members: ConsumerGroupMember[]; offsets: ConsumerGroupTopicOffset[];
}
interface ConsumerGroupMember {
  id: string; clientId: string; host: string; topics: string[];
}
interface ConsumerGroupTopicOffset {
  topic: string; partitions: ConsumerGroupPartitionOffset[]; totalLag: number;
}
interface ConsumerGroupPartitionOffset {
  partition: number; currentOffset: number; endOffset: number; lag: number;
}
interface ResetOffsetsRequest { topic: string; resetTo: string; }
```

## Frontend

### ConsumerGroupsPage

- Search input to filter groups by name
- Table columns: Name (link to detail), State (badge), Members, Topics, Coordinator
- State badge colors: `Stable` = default, `Empty` = secondary, other (Dead, PreparingRebalance, etc.) = destructive
- Empty state when no groups match

### ConsumerGroupDetailPage

- Header with group name, state badge, and "Reset Offsets" button
- Coordinator broker ID shown below header
- **Members card**: table with Member ID, Client ID, Host, assigned Topics. Shows "No active members" when empty.
- **Per-topic offset cards**: one card per subscribed topic, showing total lag badge in the header. Table with Partition, Current Offset, End Offset, Lag (destructive badge when lag > 0).
- **Reset Offsets dialog**: topic selector (populated from committed offsets), reset-to selector (Earliest/Latest), submit button with loading state and error display.

### Routes

```
/clusters/:clusterName/consumer-groups           -> ConsumerGroupsPage
/clusters/:clusterName/consumer-groups/:groupName -> ConsumerGroupDetailPage
```

## Dependencies

No new dependencies. Uses existing franz-go `kadm` package for all admin operations and existing shadcn/ui components (Badge, Card, Table, Dialog, Input, Button, Label).

## File Changes

Backend:
- `internal/kafka/client.go` -- add consumer group types and methods (`ConsumerGroups`, `ConsumerGroupDetails`, `ResetConsumerGroupOffsets`)
- `internal/api/handlers/consumer_group.go` -- new handler with `List`, `Details`, `ResetOffsets`
- `internal/api/handlers/consumer_group_test.go` -- handler unit tests
- `internal/api/router.go` -- register consumer group routes
- `internal/api/handlers/openapi.yaml` -- add consumer group endpoints and schemas

Frontend:
- `frontend/src/lib/api.ts` -- add consumer group types and API functions
- `frontend/src/pages/ConsumerGroupsPage.tsx` -- group list with search
- `frontend/src/pages/ConsumerGroupDetailPage.tsx` -- group detail with members, offsets, lag, and reset dialog
- `frontend/src/App.tsx` -- register consumer group routes
