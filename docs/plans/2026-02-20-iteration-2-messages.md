# Iteration 2: Message Browsing — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add message browsing, producing, and live tail to KafkaUI so users can read/write/stream messages from Kafka topics.

**Architecture:** REST endpoint for browse/produce, WebSocket for live tail. Backend uses franz-go's direct consumer (no consumer group) for browsing, and gorilla/websocket for live tail. Frontend adds tabs to TopicDetailPage with a message browser component.

**Tech Stack:** Go (franz-go, gorilla/websocket, chi), React (TanStack Query, native WebSocket API, shadcn/ui tabs)

---

### Task 1: Add gorilla/websocket dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add dependency**

Run: `cd /home/smyrcu/RiderProjects/KafkaUI && go get github.com/gorilla/websocket`

**Step 2: Verify**

Run: `grep gorilla go.mod`
Expected: `github.com/gorilla/websocket` present

---

### Task 2: Add ConsumeMessages to Kafka client

**Files:**
- Modify: `internal/kafka/client.go`

**Step 1: Add types and method**

Add these types after `CreateTopicRequest`:

```go
type MessageRecord struct {
	Partition int32             `json:"partition"`
	Offset    int64             `json:"offset"`
	Timestamp time.Time         `json:"timestamp"`
	Key       string            `json:"key"`
	Value     string            `json:"value"`
	Headers   map[string]string `json:"headers,omitempty"`
}

type ConsumeRequest struct {
	Partition *int32     // nil = all partitions
	Offset    int64      // -1 = latest, -2 = earliest
	Timestamp *time.Time // if set, overrides Offset
	Limit     int
}
```

Add to imports: `"time"`, `"strings"` (already there).

Add method to `Client`:

```go
func (c *Client) ConsumeMessages(ctx context.Context, topic string, req ConsumeRequest) ([]MessageRecord, error) {
	if req.Limit <= 0 {
		req.Limit = 100
	}
	if req.Limit > 500 {
		req.Limit = 500
	}

	// Get topic metadata to know partitions
	topics, err := c.admin.ListTopics(ctx, topic)
	if err != nil {
		return nil, fmt.Errorf("listing topic: %w", err)
	}
	t, ok := topics[topic]
	if !ok {
		return nil, fmt.Errorf("topic %q not found", topic)
	}

	// Build partition offsets map
	offsets := make(map[string]map[int32]kgo.Offset)
	topicOffsets := make(map[int32]kgo.Offset)

	for _, p := range t.Partitions.Sorted() {
		if req.Partition != nil && p.Partition != *req.Partition {
			continue
		}
		switch req.Offset {
		case -1:
			topicOffsets[p.Partition] = kgo.NewOffset().AtEnd()
		case -2:
			topicOffsets[p.Partition] = kgo.NewOffset().AtStart()
		default:
			topicOffsets[p.Partition] = kgo.NewOffset().At(req.Offset)
		}
	}
	offsets[topic] = topicOffsets

	// If timestamp is set, resolve offsets from timestamp
	if req.Timestamp != nil {
		ts := req.Timestamp.UnixMilli()
		listedOffsets, err := c.admin.ListOffsetsAfterMilli(ctx, ts, topic)
		if err != nil {
			return nil, fmt.Errorf("listing offsets for timestamp: %w", err)
		}
		topicOffsets = make(map[int32]kgo.Offset)
		listedOffsets.Each(func(lo kadm.ListedOffset) {
			if req.Partition != nil && lo.Partition != *req.Partition {
				return
			}
			topicOffsets[lo.Partition] = kgo.NewOffset().At(lo.Offset)
		})
		offsets[topic] = topicOffsets
	}

	if len(topicOffsets) == 0 {
		return []MessageRecord{}, nil
	}

	// Create a temporary consumer
	seeds := strings.Split(c.name, ",")
	// Re-use the existing client's seed brokers via raw client
	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(seeds...),
		kgo.ConsumePartitions(offsets),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("creating consumer: %w", err)
	}
	defer consumer.Close()

	var records []MessageRecord
	for len(records) < req.Limit {
		fetches := consumer.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			// If context canceled, return what we have
			if ctx.Err() != nil {
				break
			}
			return nil, fmt.Errorf("consuming: %v", errs[0].Err)
		}
		fetches.EachRecord(func(r *kgo.Record) {
			if len(records) >= req.Limit {
				return
			}
			headers := make(map[string]string)
			for _, h := range r.Headers {
				headers[h.Key] = string(h.Value)
			}
			records = append(records, MessageRecord{
				Partition: r.Partition,
				Offset:    r.Offset,
				Timestamp: r.Timestamp,
				Key:       string(r.Key),
				Value:     string(r.Value),
				Headers:   headers,
			})
		})
	}

	return records, nil
}
```

**Problem with above**: The consumer needs the actual seed brokers, not `c.name`. We need to store seeds in the Client struct.

**Revised approach**: Store the cluster config in Client so we can create temporary consumers.

Modify `Client` struct:

```go
type Client struct {
	raw    *kgo.Client
	admin  *kadm.Client
	name   string
	config config.ClusterConfig
}
```

Update `NewClient` to store config:

```go
return &Client{
	raw:    raw,
	admin:  kadm.NewClient(raw),
	name:   cfg.Name,
	config: cfg,
}, nil
```

Then in `ConsumeMessages`, build consumer opts from config:

```go
seeds := strings.Split(c.config.BootstrapServers, ",")
opts := []kgo.Opt{
	kgo.SeedBrokers(seeds...),
	kgo.ConsumePartitions(offsets),
}
// Re-use auth if configured
if c.config.SASL.Mechanism != "" {
	saslOpt, err := buildSASLOpt(c.config.SASL)
	if err != nil {
		return nil, fmt.Errorf("configuring SASL for consumer: %w", err)
	}
	opts = append(opts, saslOpt)
}
if c.config.TLS.Enabled {
	tlsOpt, err := buildTLSOpt(c.config.TLS)
	if err != nil {
		return nil, fmt.Errorf("configuring TLS for consumer: %w", err)
	}
	opts = append(opts, tlsOpt)
}
consumer, err := kgo.NewClient(opts...)
```

**Step 2: Verify compilation**

Run: `cd /home/smyrcu/RiderProjects/KafkaUI && go build ./...`
Expected: no errors

---

### Task 3: Add ProduceMessage to Kafka client

**Files:**
- Modify: `internal/kafka/client.go`

**Step 1: Add types and method**

Add type after `ConsumeRequest`:

```go
type ProduceRequest struct {
	Key       string            `json:"key"`
	Value     string            `json:"value"`
	Partition *int32            `json:"partition,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}
```

Add method:

```go
func (c *Client) ProduceMessage(ctx context.Context, topic string, req ProduceRequest) (*MessageRecord, error) {
	r := &kgo.Record{
		Topic: topic,
		Key:   []byte(req.Key),
		Value: []byte(req.Value),
	}
	if req.Partition != nil {
		r.Partition = *req.Partition
	} else {
		r.Partition = -1 // use default partitioner
	}
	for k, v := range req.Headers {
		r.Headers = append(r.Headers, kgo.RecordHeader{Key: k, Value: []byte(v)})
	}

	var produceErr error
	c.raw.Produce(ctx, r, func(r *kgo.Record, err error) {
		produceErr = err
	})
	if err := c.raw.Flush(ctx); err != nil {
		return nil, fmt.Errorf("flushing produce: %w", err)
	}
	if produceErr != nil {
		return nil, fmt.Errorf("producing message: %w", produceErr)
	}

	headers := make(map[string]string)
	for _, h := range r.Headers {
		headers[h.Key] = string(h.Value)
	}
	return &MessageRecord{
		Partition: r.Partition,
		Offset:    r.Offset,
		Timestamp: r.Timestamp,
		Key:       string(r.Key),
		Value:     string(r.Value),
		Headers:   headers,
	}, nil
}
```

**Note:** The existing kgo client was created without `kgo.AllowAutoTopicCreation()` and without produce-specific options. The default behavior of franz-go's `Produce` should work since it uses the same underlying client connection. If the client was created without `kgo.RecordPartitioner`, it defaults to `kgo.StickyKeyPartitioner` which is fine.

**Step 2: Verify compilation**

Run: `cd /home/smyrcu/RiderProjects/KafkaUI && go build ./...`
Expected: no errors

---

### Task 4: Create message handler (browse + produce)

**Files:**
- Create: `internal/api/handlers/message.go`
- Create: `internal/api/handlers/message_test.go`

**Step 1: Write tests**

`internal/api/handlers/message_test.go`:

```go
package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestMessageHandler_Browse_ClusterNotFound(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewMessageHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/nonexistent/topics/test/messages", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	rctx.URLParams.Add("topicName", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Browse(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestMessageHandler_Produce_ClusterNotFound(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewMessageHandler(reg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/nonexistent/topics/test/messages", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	rctx.URLParams.Add("topicName", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Produce(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /home/smyrcu/RiderProjects/KafkaUI && go test ./internal/api/handlers/ -run TestMessageHandler -v`
Expected: FAIL (compilation error, handler doesn't exist yet)

**Step 3: Write handler implementation**

`internal/api/handlers/message.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

type MessageHandler struct {
	registry *kafka.Registry
}

func NewMessageHandler(reg *kafka.Registry) *MessageHandler {
	return &MessageHandler{registry: reg}
}

func (h *MessageHandler) Browse(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")
	topicName := chi.URLParam(r, "topicName")

	client, ok := h.registry.Get(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	req := kafka.ConsumeRequest{
		Offset: -1, // latest by default
		Limit:  100,
	}

	if v := r.URL.Query().Get("partition"); v != "" {
		p, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid partition parameter")
			return
		}
		p32 := int32(p)
		req.Partition = &p32
	}

	if v := r.URL.Query().Get("offset"); v != "" {
		switch v {
		case "earliest":
			req.Offset = -2
		case "latest":
			req.Offset = -1
		default:
			o, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid offset parameter")
				return
			}
			req.Offset = o
		}
	}

	if v := r.URL.Query().Get("limit"); v != "" {
		l, err := strconv.Atoi(v)
		if err != nil || l < 1 || l > 500 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 500")
			return
		}
		req.Limit = l
	}

	if v := r.URL.Query().Get("timestamp"); v != "" {
		ts, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid timestamp (use RFC3339 format)")
			return
		}
		req.Timestamp = &ts
	}

	// Use a timeout context so we don't hang forever on empty topics
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	messages, err := client.ConsumeMessages(ctx, topicName, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, messages)
}

func (h *MessageHandler) Produce(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")
	topicName := chi.URLParam(r, "topicName")

	client, ok := h.registry.Get(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	var req kafka.ProduceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	record, err := client.ProduceMessage(r.Context(), topicName, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, record)
}
```

Add `"context"` to imports.

**Step 4: Run tests to verify they pass**

Run: `cd /home/smyrcu/RiderProjects/KafkaUI && go test ./internal/api/handlers/ -run TestMessageHandler -v`
Expected: PASS

---

### Task 5: Create WebSocket live tail handler

**Files:**
- Create: `internal/api/ws/livetail.go`

**Step 1: Create ws directory and handler**

`internal/api/ws/livetail.go`:

```go
package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type wsMessage struct {
	Partition int32             `json:"partition"`
	Offset    int64             `json:"offset"`
	Timestamp time.Time         `json:"timestamp"`
	Key       string            `json:"key"`
	Value     string            `json:"value"`
	Headers   map[string]string `json:"headers,omitempty"`
}

type controlMessage struct {
	Action string `json:"action"` // "start" or "stop"
}

type LiveTailHandler struct {
	registry *kafka.Registry
	logger   *slog.Logger
}

func NewLiveTailHandler(registry *kafka.Registry, logger *slog.Logger) *LiveTailHandler {
	return &LiveTailHandler{registry: registry, logger: logger}
}

func (h *LiveTailHandler) Handle(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")
	topicName := chi.URLParam(r, "topicName")

	client, ok := h.registry.Get(clusterName)
	if !ok {
		http.Error(w, "cluster not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	cfg := client.Config()
	seeds := strings.Split(cfg.BootstrapServers, ",")
	opts := []kgo.Opt{
		kgo.SeedBrokers(seeds...),
		kgo.ConsumeTopics(topicName),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
	}
	if cfg.SASL.Mechanism != "" {
		saslOpt, saslErr := kafka.BuildSASLOpt(cfg.SASL)
		if saslErr != nil {
			h.logger.Error("SASL config failed for live tail", "error", saslErr)
			return
		}
		opts = append(opts, saslOpt)
	}
	if cfg.TLS.Enabled {
		tlsOpt, tlsErr := kafka.BuildTLSOpt(cfg.TLS)
		if tlsErr != nil {
			h.logger.Error("TLS config failed for live tail", "error", tlsErr)
			return
		}
		opts = append(opts, tlsOpt)
	}

	consumer, err := kgo.NewClient(opts...)
	if err != nil {
		h.logger.Error("failed to create live tail consumer", "error", err)
		return
	}
	defer consumer.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Read control messages from client
	go func() {
		defer cancel()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var ctrl controlMessage
			if json.Unmarshal(msg, &ctrl) == nil && ctrl.Action == "stop" {
				return
			}
		}
	}()

	// Set up ping/pong
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		default:
			fetchCtx, fetchCancel := context.WithTimeout(ctx, 2*time.Second)
			fetches := consumer.PollFetches(fetchCtx)
			fetchCancel()

			if fetchCtx.Err() != nil && ctx.Err() != nil {
				return
			}

			fetches.EachRecord(func(r *kgo.Record) {
				headers := make(map[string]string)
				for _, hdr := range r.Headers {
					headers[hdr.Key] = string(hdr.Value)
				}
				msg := wsMessage{
					Partition: r.Partition,
					Offset:    r.Offset,
					Timestamp: r.Timestamp,
					Key:       string(r.Key),
					Value:     string(r.Value),
					Headers:   headers,
				}
				data, _ := json.Marshal(msg)
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					cancel()
				}
			})
		}
	}
}
```

**Step 2: Export auth builder functions from kafka package**

Modify `internal/kafka/auth.go` — rename `buildSASLOpt` → `BuildSASLOpt` and `buildTLSOpt` → `BuildTLSOpt` (exported).

Also update `internal/kafka/client.go` references from `buildSASLOpt` → `BuildSASLOpt` and `buildTLSOpt` → `BuildTLSOpt`.

Add `Config()` method to `Client`:

```go
func (c *Client) Config() config.ClusterConfig {
	return c.config
}
```

**Step 3: Verify compilation**

Run: `cd /home/smyrcu/RiderProjects/KafkaUI && go build ./...`
Expected: no errors

---

### Task 6: Register new routes in router

**Files:**
- Modify: `internal/api/router.go`

**Step 1: Add message and WebSocket routes**

Update imports to include ws package. Add handler instantiation and routes:

```go
import (
	// ... existing imports
	"github.com/Smyrcu/KafkaUI/internal/api/ws"
)

// In NewRouter function, after topicHandler:
messageHandler := handlers.NewMessageHandler(registry)
liveTailHandler := ws.NewLiveTailHandler(registry, logger)

// Inside r.Route("/clusters/{clusterName}", ...), after topic routes:
r.Get("/topics/{topicName}/messages", messageHandler.Browse)
r.Post("/topics/{topicName}/messages", messageHandler.Produce)

// Add WebSocket route outside the /api/v1 group:
r.Route("/ws", func(r chi.Router) {
	r.Get("/clusters/{clusterName}/topics/{topicName}/live", liveTailHandler.Handle)
})
```

**Step 2: Verify compilation**

Run: `cd /home/smyrcu/RiderProjects/KafkaUI && go build ./...`
Expected: no errors

**Step 3: Run all backend tests**

Run: `cd /home/smyrcu/RiderProjects/KafkaUI && go test ./... -v`
Expected: all pass

---

### Task 7: Update OpenAPI spec

**Files:**
- Modify: `internal/api/handlers/openapi.yaml`

**Step 1: Add message endpoints and schemas**

Add to `paths:` section:

```yaml
  /clusters/{clusterName}/topics/{topicName}/messages:
    get:
      summary: Browse messages
      description: Consume messages from a topic with offset/timestamp filtering
      operationId: browseMessages
      tags: [Messages]
      parameters:
        - $ref: "#/components/parameters/ClusterName"
        - $ref: "#/components/parameters/TopicName"
        - name: partition
          in: query
          schema:
            type: integer
          description: Partition to consume from (all if omitted)
        - name: offset
          in: query
          schema:
            type: string
            default: "latest"
          description: "Offset to start from: 'earliest', 'latest', or a number"
        - name: limit
          in: query
          schema:
            type: integer
            minimum: 1
            maximum: 500
            default: 100
          description: Maximum number of messages to return
        - name: timestamp
          in: query
          schema:
            type: string
            format: date-time
          description: Seek to this timestamp (RFC3339, overrides offset)
      responses:
        "200":
          description: List of messages
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/MessageRecord"
        "400":
          $ref: "#/components/responses/BadRequest"
        "404":
          $ref: "#/components/responses/NotFound"
        "500":
          $ref: "#/components/responses/InternalError"
    post:
      summary: Produce message
      description: Produce a message to a topic
      operationId: produceMessage
      tags: [Messages]
      parameters:
        - $ref: "#/components/parameters/ClusterName"
        - $ref: "#/components/parameters/TopicName"
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/ProduceRequest"
      responses:
        "200":
          description: Message produced
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/MessageRecord"
        "400":
          $ref: "#/components/responses/BadRequest"
        "404":
          $ref: "#/components/responses/NotFound"
        "500":
          $ref: "#/components/responses/InternalError"
```

Add to `components.schemas`:

```yaml
    MessageRecord:
      type: object
      properties:
        partition:
          type: integer
        offset:
          type: integer
          format: int64
        timestamp:
          type: string
          format: date-time
        key:
          type: string
        value:
          type: string
        headers:
          type: object
          additionalProperties:
            type: string

    ProduceRequest:
      type: object
      properties:
        key:
          type: string
        value:
          type: string
        partition:
          type: integer
          nullable: true
        headers:
          type: object
          additionalProperties:
            type: string
```

---

### Task 8: Install shadcn tabs component

**Files:**
- Create: `frontend/src/components/ui/tabs.tsx`

**Step 1: Add radix tabs dependency**

Run: `cd /home/smyrcu/RiderProjects/KafkaUI/frontend && npm install @radix-ui/react-tabs`

**Step 2: Create tabs component**

`frontend/src/components/ui/tabs.tsx`:

```tsx
import * as React from "react"
import * as TabsPrimitive from "@radix-ui/react-tabs"
import { cn } from "@/lib/utils"

const Tabs = TabsPrimitive.Root

const TabsList = React.forwardRef<
  React.ComponentRef<typeof TabsPrimitive.List>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.List>
>(({ className, ...props }, ref) => (
  <TabsPrimitive.List
    ref={ref}
    className={cn(
      "inline-flex h-10 items-center justify-center rounded-md bg-muted p-1 text-muted-foreground",
      className
    )}
    {...props}
  />
))
TabsList.displayName = TabsPrimitive.List.displayName

const TabsTrigger = React.forwardRef<
  React.ComponentRef<typeof TabsPrimitive.Trigger>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.Trigger>
>(({ className, ...props }, ref) => (
  <TabsPrimitive.Trigger
    ref={ref}
    className={cn(
      "inline-flex items-center justify-center whitespace-nowrap rounded-sm px-3 py-1.5 text-sm font-medium ring-offset-background transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-sm",
      className
    )}
    {...props}
  />
))
TabsTrigger.displayName = TabsPrimitive.Trigger.displayName

const TabsContent = React.forwardRef<
  React.ComponentRef<typeof TabsPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.Content>
>(({ className, ...props }, ref) => (
  <TabsPrimitive.Content
    ref={ref}
    className={cn(
      "mt-2 ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
      className
    )}
    {...props}
  />
))
TabsContent.displayName = TabsPrimitive.Content.displayName

export { Tabs, TabsList, TabsTrigger, TabsContent }
```

**Step 3: Verify build**

Run: `cd /home/smyrcu/RiderProjects/KafkaUI/frontend && npm run build`
Expected: no errors

---

### Task 9: Add message types and API functions to frontend

**Files:**
- Modify: `frontend/src/lib/api.ts`

**Step 1: Add types and API functions**

Add interfaces:

```typescript
export interface MessageRecord {
  partition: number;
  offset: number;
  timestamp: string;
  key: string;
  value: string;
  headers?: Record<string, string>;
}

export interface ProduceRequest {
  key: string;
  value: string;
  partition?: number | null;
  headers?: Record<string, string>;
}

export interface BrowseParams {
  partition?: number;
  offset?: string;
  limit?: number;
  timestamp?: string;
}
```

Add to `api` object:

```typescript
messages: {
  browse: (cluster: string, topic: string, params?: BrowseParams) => {
    const searchParams = new URLSearchParams();
    if (params?.partition !== undefined) searchParams.set('partition', String(params.partition));
    if (params?.offset) searchParams.set('offset', params.offset);
    if (params?.limit) searchParams.set('limit', String(params.limit));
    if (params?.timestamp) searchParams.set('timestamp', params.timestamp);
    const qs = searchParams.toString();
    return request<MessageRecord[]>(`/clusters/${cluster}/topics/${topic}/messages${qs ? `?${qs}` : ''}`);
  },
  produce: (cluster: string, topic: string, data: ProduceRequest) =>
    request<MessageRecord>(`/clusters/${cluster}/topics/${topic}/messages`, { method: 'POST', body: JSON.stringify(data) }),
},
```

---

### Task 10: Create useWebSocket hook

**Files:**
- Create: `frontend/src/hooks/useWebSocket.ts`

```typescript
import { useState, useEffect, useRef, useCallback } from 'react';
import type { MessageRecord } from '@/lib/api';

type ConnectionState = 'disconnected' | 'connecting' | 'connected';

export function useWebSocket(url: string | null) {
  const [messages, setMessages] = useState<MessageRecord[]>([]);
  const [state, setState] = useState<ConnectionState>('disconnected');
  const wsRef = useRef<WebSocket | null>(null);

  const connect = useCallback(() => {
    if (!url || wsRef.current) return;

    setState('connecting');
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}${url}`;
    const ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      setState('connected');
      ws.send(JSON.stringify({ action: 'start' }));
    };

    ws.onmessage = (event) => {
      const msg: MessageRecord = JSON.parse(event.data);
      setMessages((prev) => [msg, ...prev].slice(0, 1000));
    };

    ws.onclose = () => {
      setState('disconnected');
      wsRef.current = null;
    };

    ws.onerror = () => {
      ws.close();
    };

    wsRef.current = ws;
  }, [url]);

  const disconnect = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.send(JSON.stringify({ action: 'stop' }));
      wsRef.current.close();
      wsRef.current = null;
    }
    setState('disconnected');
  }, []);

  const clear = useCallback(() => {
    setMessages([]);
  }, []);

  useEffect(() => {
    return () => {
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, []);

  return { messages, state, connect, disconnect, clear };
}
```

---

### Task 11: Refactor TopicDetailPage to tabbed layout

**Files:**
- Modify: `frontend/src/pages/TopicDetailPage.tsx`
- Modify: `frontend/src/App.tsx`

**Step 1: Add Messages route**

In `App.tsx`, add import and route:

```typescript
import { TopicMessagesPage } from "@/pages/TopicMessagesPage";
```

Add route after topic detail route:

```tsx
<Route path="/clusters/:clusterName/topics/:topicName/messages" element={<TopicMessagesPage />} />
```

**Step 2: Refactor TopicDetailPage to include tab navigation**

Replace `TopicDetailPage.tsx` content:

```tsx
import { useQuery } from "@tanstack/react-query";
import { useParams, useLocation, Link } from "react-router-dom";
import { api } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { ErrorAlert } from "@/components/ErrorAlert";
import { cn } from "@/lib/utils";

export function TopicDetailPage() {
  const { clusterName, topicName } = useParams<{ clusterName: string; topicName: string }>();
  const location = useLocation();

  const { data: topic, isLoading, error } = useQuery({
    queryKey: ["topic", clusterName, topicName],
    queryFn: () => api.topics.details(clusterName!, topicName!),
    enabled: !!clusterName && !!topicName,
  });

  if (isLoading) return <div className="text-muted-foreground">Loading topic details...</div>;
  if (error) return <ErrorAlert message={(error as Error).message} />;
  if (!topic) return null;

  const basePath = `/clusters/${clusterName}/topics/${topicName}`;
  const isMessages = location.pathname.endsWith("/messages");

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <h2 className="text-2xl font-bold">{topic.name}</h2>
        {topic.internal && <Badge variant="secondary">internal</Badge>}
      </div>
      <div className="flex gap-1 border-b">
        <Link
          to={basePath}
          className={cn(
            "px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors",
            !isMessages
              ? "border-primary text-foreground"
              : "border-transparent text-muted-foreground hover:text-foreground"
          )}
        >
          Details
        </Link>
        <Link
          to={`${basePath}/messages`}
          className={cn(
            "px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors",
            isMessages
              ? "border-primary text-foreground"
              : "border-transparent text-muted-foreground hover:text-foreground"
          )}
        >
          Messages
        </Link>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>Partitions</CardTitle></CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Partition</TableHead><TableHead>Leader</TableHead><TableHead>Replicas</TableHead><TableHead>ISR</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {topic.partitions.map((p) => (
                  <TableRow key={p.id}>
                    <TableCell><Badge variant="outline">{p.id}</Badge></TableCell>
                    <TableCell>{p.leader}</TableCell>
                    <TableCell>{p.replicas.join(", ")}</TableCell>
                    <TableCell>{p.isr.join(", ")}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>Configuration</CardTitle></CardHeader>
          <CardContent>
            <Table>
              <TableHeader><TableRow><TableHead>Key</TableHead><TableHead>Value</TableHead></TableRow></TableHeader>
              <TableBody>
                {Object.entries(topic.configs).map(([key, value]) => (
                  <TableRow key={key}>
                    <TableCell className="font-mono text-xs">{key}</TableCell>
                    <TableCell className="font-mono text-xs">{value}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
```

Wait — this approach is wrong. The Details tab shows the existing content, and the Messages tab should show a different page. Better approach: use the router. TopicDetailPage shows details (with tab nav), TopicMessagesPage shows messages (with same tab nav). Extract the tab nav into a shared component or just duplicate it (simpler, YAGNI).

**Revised approach**: Create a `TopicTabs` component for the shared tab navigation, used by both pages.

Create `frontend/src/components/TopicTabs.tsx`:

```tsx
import { Link, useParams, useLocation } from "react-router-dom";
import { cn } from "@/lib/utils";

export function TopicTabs() {
  const { clusterName, topicName } = useParams();
  const location = useLocation();
  const basePath = `/clusters/${clusterName}/topics/${topicName}`;
  const isMessages = location.pathname.endsWith("/messages");

  return (
    <div className="flex gap-1 border-b mb-6">
      <Link
        to={basePath}
        className={cn(
          "px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors",
          !isMessages
            ? "border-primary text-foreground"
            : "border-transparent text-muted-foreground hover:text-foreground"
        )}
      >
        Details
      </Link>
      <Link
        to={`${basePath}/messages`}
        className={cn(
          "px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors",
          isMessages
            ? "border-primary text-foreground"
            : "border-transparent text-muted-foreground hover:text-foreground"
        )}
      >
        Messages
      </Link>
    </div>
  );
}
```

Then update `TopicDetailPage.tsx` — add `<TopicTabs />` after the title:

```tsx
import { TopicTabs } from "@/components/TopicTabs";
// ... after the title div:
<TopicTabs />
```

And use `<TopicTabs />` in the new `TopicMessagesPage` too.

**Step 3: Verify build**

Run: `cd /home/smyrcu/RiderProjects/KafkaUI/frontend && npm run build`
Expected: no errors

---

### Task 12: Create TopicMessagesPage

**Files:**
- Create: `frontend/src/pages/TopicMessagesPage.tsx`

This is the main message browser page with:
- Toolbar with partition/offset/limit controls and Fetch button
- Live Tail toggle
- Produce Message button + dialog
- Message table with expandable rows

```tsx
import { useState } from "react";
import { useParams } from "react-router-dom";
import { useQuery, useMutation } from "@tanstack/react-query";
import { api, type BrowseParams, type ProduceRequest, type MessageRecord } from "@/lib/api";
import { useWebSocket } from "@/hooks/useWebSocket";
import { TopicTabs } from "@/components/TopicTabs";
import { Card, CardContent } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger, DialogFooter } from "@/components/ui/dialog";
import { ErrorAlert } from "@/components/ErrorAlert";
import { Play, Square, Send, Plus, Trash2, ChevronDown, ChevronRight } from "lucide-react";

type OffsetMode = "latest" | "earliest" | "timestamp" | "custom";

export function TopicMessagesPage() {
  const { clusterName, topicName } = useParams<{ clusterName: string; topicName: string }>();

  // Browse state
  const [partition, setPartition] = useState<string>("");
  const [offsetMode, setOffsetMode] = useState<OffsetMode>("latest");
  const [customOffset, setCustomOffset] = useState("");
  const [timestamp, setTimestamp] = useState("");
  const [limit, setLimit] = useState(100);
  const [browseParams, setBrowseParams] = useState<BrowseParams | null>(null);
  const [expandedRow, setExpandedRow] = useState<string | null>(null);

  // Live tail
  const wsUrl = clusterName && topicName
    ? `/ws/clusters/${clusterName}/topics/${topicName}/live`
    : null;
  const { messages: liveMessages, state: wsState, connect, disconnect, clear } = useWebSocket(wsUrl);
  const isLiveTail = wsState === "connected" || wsState === "connecting";

  // Produce state
  const [produceOpen, setProduceOpen] = useState(false);
  const [produceData, setProduceData] = useState<ProduceRequest>({ key: "", value: "" });
  const [produceHeaders, setProduceHeaders] = useState<{ key: string; value: string }[]>([]);

  // Fetch messages
  const { data: fetchedMessages, isLoading, error } = useQuery({
    queryKey: ["messages", clusterName, topicName, browseParams],
    queryFn: () => api.messages.browse(clusterName!, topicName!, browseParams!),
    enabled: !!clusterName && !!topicName && !!browseParams && !isLiveTail,
  });

  const produceMutation = useMutation({
    mutationFn: (data: ProduceRequest) => api.messages.produce(clusterName!, topicName!, data),
    onSuccess: () => {
      setProduceOpen(false);
      setProduceData({ key: "", value: "" });
      setProduceHeaders([]);
    },
  });

  const handleFetch = () => {
    const params: BrowseParams = { limit };
    if (partition !== "") params.partition = parseInt(partition);
    switch (offsetMode) {
      case "latest": params.offset = "latest"; break;
      case "earliest": params.offset = "earliest"; break;
      case "custom": params.offset = customOffset; break;
      case "timestamp": params.timestamp = timestamp; break;
    }
    setBrowseParams({ ...params });
  };

  const handleProduce = () => {
    const headers: Record<string, string> = {};
    produceHeaders.forEach((h) => { if (h.key) headers[h.key] = h.value; });
    produceMutation.mutate({
      ...produceData,
      headers: Object.keys(headers).length > 0 ? headers : undefined,
    });
  };

  const toggleLiveTail = () => {
    if (isLiveTail) {
      disconnect();
    } else {
      setBrowseParams(null);
      connect();
    }
  };

  const messages = isLiveTail ? liveMessages : (fetchedMessages || []);
  const rowKey = (m: MessageRecord) => `${m.partition}-${m.offset}`;

  const tryFormatJson = (s: string) => {
    try { return JSON.stringify(JSON.parse(s), null, 2); } catch { return s; }
  };

  return (
    <div>
      <div className="flex items-center gap-3 mb-2">
        <h2 className="text-2xl font-bold">{topicName}</h2>
      </div>
      <TopicTabs />

      {/* Toolbar */}
      <div className="flex flex-wrap items-end gap-3 mb-4">
        <div className="grid gap-1">
          <Label className="text-xs">Partition</Label>
          <Input className="w-24" placeholder="All" value={partition} onChange={(e) => setPartition(e.target.value)} disabled={isLiveTail} />
        </div>
        <div className="grid gap-1">
          <Label className="text-xs">Offset</Label>
          <select
            className="h-9 rounded-md border border-input bg-background px-3 text-sm"
            value={offsetMode}
            onChange={(e) => setOffsetMode(e.target.value as OffsetMode)}
            disabled={isLiveTail}
          >
            <option value="latest">Latest</option>
            <option value="earliest">Earliest</option>
            <option value="timestamp">Timestamp</option>
            <option value="custom">Custom</option>
          </select>
        </div>
        {offsetMode === "custom" && (
          <div className="grid gap-1">
            <Label className="text-xs">Offset value</Label>
            <Input className="w-28" value={customOffset} onChange={(e) => setCustomOffset(e.target.value)} disabled={isLiveTail} />
          </div>
        )}
        {offsetMode === "timestamp" && (
          <div className="grid gap-1">
            <Label className="text-xs">Timestamp</Label>
            <Input type="datetime-local" className="w-52" value={timestamp} onChange={(e) => setTimestamp(e.target.value)} disabled={isLiveTail} />
          </div>
        )}
        <div className="grid gap-1">
          <Label className="text-xs">Limit</Label>
          <Input className="w-20" type="number" min={1} max={500} value={limit} onChange={(e) => setLimit(parseInt(e.target.value) || 100)} disabled={isLiveTail} />
        </div>
        <Button onClick={handleFetch} disabled={isLiveTail || isLoading}>
          {isLoading ? "Loading..." : "Fetch"}
        </Button>
        <Button variant={isLiveTail ? "destructive" : "outline"} onClick={toggleLiveTail}>
          {isLiveTail ? <><Square className="h-4 w-4 mr-2" />Stop</> : <><Play className="h-4 w-4 mr-2" />Live Tail</>}
        </Button>
        {isLiveTail && (
          <Button variant="ghost" size="sm" onClick={clear}>Clear ({liveMessages.length})</Button>
        )}
        <div className="ml-auto">
          <Dialog open={produceOpen} onOpenChange={setProduceOpen}>
            <DialogTrigger asChild>
              <Button variant="outline"><Send className="h-4 w-4 mr-2" />Produce</Button>
            </DialogTrigger>
            <DialogContent className="max-w-lg">
              <DialogHeader><DialogTitle>Produce Message</DialogTitle></DialogHeader>
              <div className="grid gap-4 py-2">
                <div className="grid gap-2">
                  <Label>Key</Label>
                  <Input value={produceData.key} onChange={(e) => setProduceData({ ...produceData, key: e.target.value })} placeholder="Message key (optional)" />
                </div>
                <div className="grid gap-2">
                  <Label>Value</Label>
                  <textarea
                    className="flex min-h-[120px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring font-mono"
                    value={produceData.value}
                    onChange={(e) => setProduceData({ ...produceData, value: e.target.value })}
                    placeholder='{"event": "test"}'
                  />
                </div>
                <div className="grid gap-2">
                  <div className="flex items-center justify-between">
                    <Label>Headers</Label>
                    <Button variant="ghost" size="sm" onClick={() => setProduceHeaders([...produceHeaders, { key: "", value: "" }])}>
                      <Plus className="h-3 w-3 mr-1" />Add
                    </Button>
                  </div>
                  {produceHeaders.map((h, i) => (
                    <div key={i} className="flex gap-2">
                      <Input placeholder="Key" value={h.key} onChange={(e) => { const nh = [...produceHeaders]; nh[i].key = e.target.value; setProduceHeaders(nh); }} />
                      <Input placeholder="Value" value={h.value} onChange={(e) => { const nh = [...produceHeaders]; nh[i].value = e.target.value; setProduceHeaders(nh); }} />
                      <Button variant="ghost" size="icon" onClick={() => setProduceHeaders(produceHeaders.filter((_, j) => j !== i))}>
                        <Trash2 className="h-3 w-3" />
                      </Button>
                    </div>
                  ))}
                </div>
              </div>
              <DialogFooter>
                <Button onClick={handleProduce} disabled={produceMutation.isPending}>
                  {produceMutation.isPending ? "Sending..." : "Send"}
                </Button>
              </DialogFooter>
              {produceMutation.isError && <p className="text-sm text-destructive">{(produceMutation.error as Error).message}</p>}
            </DialogContent>
          </Dialog>
        </div>
      </div>

      {/* Connection indicator */}
      {isLiveTail && (
        <div className="flex items-center gap-2 mb-3">
          <span className="relative flex h-2 w-2">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
            <span className="relative inline-flex rounded-full h-2 w-2 bg-green-500"></span>
          </span>
          <span className="text-xs text-muted-foreground">Live tail active — {liveMessages.length} messages</span>
        </div>
      )}

      {error && <ErrorAlert message={(error as Error).message} />}

      {/* Message table */}
      {messages.length > 0 ? (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-8"></TableHead>
              <TableHead>Partition</TableHead>
              <TableHead>Offset</TableHead>
              <TableHead>Timestamp</TableHead>
              <TableHead>Key</TableHead>
              <TableHead>Value</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {messages.map((m) => {
              const key = rowKey(m);
              const isExpanded = expandedRow === key;
              return (
                <>
                  <TableRow key={key} className="cursor-pointer hover:bg-muted/50" onClick={() => setExpandedRow(isExpanded ? null : key)}>
                    <TableCell>{isExpanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}</TableCell>
                    <TableCell><Badge variant="outline">{m.partition}</Badge></TableCell>
                    <TableCell className="font-mono text-xs">{m.offset}</TableCell>
                    <TableCell className="text-xs">{new Date(m.timestamp).toLocaleString()}</TableCell>
                    <TableCell className="font-mono text-xs max-w-[150px] truncate">{m.key || <span className="text-muted-foreground">null</span>}</TableCell>
                    <TableCell className="font-mono text-xs max-w-[300px] truncate">{m.value}</TableCell>
                  </TableRow>
                  {isExpanded && (
                    <TableRow key={`${key}-detail`}>
                      <TableCell colSpan={6} className="bg-muted/30 p-4">
                        <div className="grid gap-3">
                          <div>
                            <p className="text-xs font-semibold mb-1">Key</p>
                            <pre className="text-xs bg-background rounded p-2 border overflow-auto max-h-32">{m.key ? tryFormatJson(m.key) : "null"}</pre>
                          </div>
                          <div>
                            <p className="text-xs font-semibold mb-1">Value</p>
                            <pre className="text-xs bg-background rounded p-2 border overflow-auto max-h-64">{tryFormatJson(m.value)}</pre>
                          </div>
                          {m.headers && Object.keys(m.headers).length > 0 && (
                            <div>
                              <p className="text-xs font-semibold mb-1">Headers</p>
                              <pre className="text-xs bg-background rounded p-2 border overflow-auto">{JSON.stringify(m.headers, null, 2)}</pre>
                            </div>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  )}
                </>
              );
            })}
          </TableBody>
        </Table>
      ) : (
        !isLoading && !isLiveTail && (
          <Card>
            <CardContent className="flex items-center justify-center py-12 text-muted-foreground">
              Click "Fetch" to browse messages or "Live Tail" to stream new messages
            </CardContent>
          </Card>
        )
      )}
    </div>
  );
}
```

---

### Task 13: Wire up routes and verify frontend build

**Files:**
- Modify: `frontend/src/App.tsx`

**Step 1: Add import and route**

Add import:
```typescript
import { TopicMessagesPage } from "@/pages/TopicMessagesPage";
```

Add route after the topic detail route:
```tsx
<Route path="/clusters/:clusterName/topics/:topicName/messages" element={<TopicMessagesPage />} />
```

**Step 2: Update TopicDetailPage to include TopicTabs**

Modify `frontend/src/pages/TopicDetailPage.tsx` — add import and component:

```tsx
import { TopicTabs } from "@/components/TopicTabs";
```

Add `<TopicTabs />` after the title+badge div and before the grid.

**Step 3: Verify full frontend build**

Run: `cd /home/smyrcu/RiderProjects/KafkaUI/frontend && npm run build`
Expected: no errors

---

### Task 14: Adjust server WriteTimeout for WebSocket and SSE

**Files:**
- Modify: `cmd/kafkaui/main.go`

The current `WriteTimeout: 15 * time.Second` will kill WebSocket connections. Set `WriteTimeout: 0` to disable it (WebSocket connections are long-lived).

Change:
```go
WriteTimeout: 15 * time.Second,
```
To:
```go
WriteTimeout: 0,
```

**Verify compilation:**

Run: `cd /home/smyrcu/RiderProjects/KafkaUI && go build ./...`
Expected: no errors

---

### Task 15: Full build and smoke test

**Step 1: Build backend**

Run: `cd /home/smyrcu/RiderProjects/KafkaUI && go build ./...`
Expected: no errors

**Step 2: Run backend tests**

Run: `cd /home/smyrcu/RiderProjects/KafkaUI && go test ./... -v`
Expected: all pass

**Step 3: Build frontend**

Run: `cd /home/smyrcu/RiderProjects/KafkaUI/frontend && npm run build`
Expected: no errors

**Step 4: Lint frontend**

Run: `cd /home/smyrcu/RiderProjects/KafkaUI/frontend && npm run lint`
Expected: no errors (or only warnings)
