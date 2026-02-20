# Iteration 2: Message Browsing — Design

## Overview

Add message browsing, producing, and live tail capabilities to KafkaUI. Users can browse messages from any topic with offset/timestamp filters, produce new messages, and watch messages arrive in real-time via WebSocket.

## Backend API

### Browse Messages

`GET /api/v1/clusters/{clusterName}/topics/{topicName}/messages`

Query parameters:
- `partition` — integer, optional (all partitions if omitted)
- `offset` — `earliest`, `latest`, or integer (default: `latest`)
- `limit` — integer 1-500 (default: 100)
- `timestamp` — ISO8601 datetime, seeks to this timestamp (overrides offset)

Response: JSON array of messages:
```json
[
  {
    "partition": 0,
    "offset": 42,
    "timestamp": "2026-02-20T10:30:00Z",
    "key": "user-123",
    "value": "{\"name\":\"John\"}",
    "headers": {"correlationId": "abc-123"}
  }
]
```

Implementation: Create a temporary franz-go consumer group-less client, seek to requested position, read up to `limit` records, close client. Key and value are returned as strings (base64 for binary data).

### Produce Message

`POST /api/v1/clusters/{clusterName}/topics/{topicName}/messages`

Request body:
```json
{
  "key": "user-123",
  "value": "{\"event\":\"login\"}",
  "partition": null,
  "headers": {"correlationId": "abc-123"}
}
```

Implementation: Synchronous produce via the existing kgo client. Partition is optional (null = default partitioner).

### Live Tail (WebSocket)

`GET /ws/clusters/{clusterName}/topics/{topicName}/live`

- Upgrades HTTP to WebSocket via gorilla/websocket
- Starts consuming from `latest` offset on all partitions
- Streams messages as JSON objects to the client
- Client sends control messages: `{"action":"start"}`, `{"action":"stop"}`
- Auto-cleanup on disconnect (close consumer, release resources)
- Heartbeat ping/pong every 30 seconds

## Frontend

### TopicDetailPage Refactor

Convert TopicDetailPage into a tabbed layout:
- **Details** tab — existing partitions & config view
- **Messages** tab — new message browser

### Messages Tab

**Toolbar:**
- Partition selector (dropdown: All, 0, 1, 2...)
- Offset mode (radio: Latest, Earliest, Timestamp, Custom)
- Timestamp picker (shown when mode = Timestamp)
- Custom offset input (shown when mode = Custom)
- Limit input (default 100)
- "Fetch" button
- "Live Tail" toggle button
- "Produce Message" button

**Message Table:**
- Columns: Partition, Offset, Timestamp, Key (truncated), Value (truncated)
- Click row to expand full content
- JSON values are pretty-printed with syntax highlighting (simple pre/code block)
- Auto-scroll when live tail is active

**Produce Dialog:**
- Key input (textarea)
- Value input (textarea, larger)
- Partition input (optional number)
- Headers (key-value pairs, add/remove rows)
- "Send" button

### WebSocket Client

New utility in `lib/ws.ts`:
- Connect/disconnect to live tail endpoint
- Parse incoming messages
- Reconnect logic on unexpected disconnect
- Hook: `useWebSocket(url)` returning messages array and connection state

## Dependencies

Backend:
- `github.com/gorilla/websocket` — WebSocket support

Frontend:
- No new dependencies (native WebSocket API)

## File Changes

Backend:
- `internal/kafka/client.go` — add `ConsumeMessages()`, `ProduceMessage()` methods
- `internal/api/handlers/message.go` — new handler for browse + produce
- `internal/api/ws/livetail.go` — new WebSocket handler for live tail
- `internal/api/router.go` — register new routes
- `internal/api/handlers/openapi.yaml` — add message endpoints
- `go.mod` — add gorilla/websocket

Frontend:
- `frontend/src/lib/api.ts` — add message API functions
- `frontend/src/lib/ws.ts` — new WebSocket utility
- `frontend/src/hooks/useWebSocket.ts` — new hook
- `frontend/src/pages/TopicDetailPage.tsx` — refactor to tabbed layout
- `frontend/src/pages/TopicMessagesPage.tsx` — new message browser component
- `frontend/src/components/ui/tabs.tsx` — add shadcn tabs component
