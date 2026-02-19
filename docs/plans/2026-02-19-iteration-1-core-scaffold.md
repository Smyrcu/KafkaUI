# Iteration 1: Core Scaffold — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the project foundation — Go backend with config parsing, Kafka client, REST API for clusters/brokers/topics with CRUD, React frontend with shadcn/ui showing cluster overview, broker list, and topic management.

**Architecture:** Go monorepo with embedded React frontend. chi router serves REST API under `/api/v1/`, static frontend files served at root via `go:embed`. franz-go for Kafka admin operations. Config loaded from YAML file with env var expansion.

**Tech Stack:** Go 1.25, franz-go, chi, slog | React 19, Vite, shadcn/ui, Tailwind CSS, TanStack Query, React Router v7

**Reference:** See `docs/plans/2026-02-19-kafka-ui-design.md` for full architecture.

---

### Task 1: Go module and directory structure

**Files:**
- Create: `cmd/kafkaui/main.go`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/kafka/client.go`
- Create: `internal/api/router.go`
- Create: `internal/api/handlers/cluster.go`
- Create: `internal/api/handlers/broker.go`
- Create: `internal/api/handlers/topic.go`
- Create: `internal/api/middleware/logging.go`
- Create: `config.example.yaml`
- Create: `Makefile`
- Create: `.gitignore`

**Step 1: Initialize Go module**

```bash
cd /home/smyrcu/RiderProjects/KafkaUI
go mod init github.com/Smyrcu/KafkaUI
```

**Step 2: Create directory structure**

```bash
mkdir -p cmd/kafkaui
mkdir -p internal/{config,kafka,api/{handlers,middleware,ws}}
mkdir -p frontend
```

**Step 3: Install Go dependencies**

```bash
go get github.com/go-chi/chi/v5
go get github.com/go-chi/cors
go get github.com/twmb/franz-go/pkg/kgo
go get github.com/twmb/franz-go/pkg/kadm
go get gopkg.in/yaml.v3
```

**Step 4: Create `.gitignore`**

```gitignore
# Go
/kafkaui
*.exe
*.test
*.out

# Frontend
frontend/node_modules/
frontend/dist/

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store
Thumbs.db

# Config with secrets
config.yaml
!config.example.yaml
```

**Step 5: Create `config.example.yaml`**

```yaml
server:
  port: 8080
  base-path: ""

auth:
  enabled: false

clusters:
  - name: local
    bootstrap-servers: localhost:9092
```

**Step 6: Create `Makefile`**

```makefile
.PHONY: dev dev-backend dev-frontend build build-frontend build-backend test test-backend test-frontend docker clean

# Development
dev:
	@echo "Starting development servers..."
	$(MAKE) dev-backend & $(MAKE) dev-frontend & wait

dev-backend:
	go run ./cmd/kafkaui --config config.yaml

dev-frontend:
	cd frontend && npm run dev

# Build
build: build-frontend build-backend

build-frontend:
	cd frontend && npm ci && npm run build

build-backend:
	CGO_ENABLED=0 go build -o kafkaui ./cmd/kafkaui

# Test
test: test-backend test-frontend

test-backend:
	go test ./... -v

test-frontend:
	cd frontend && npm test

# Docker
docker:
	docker build -t kafkaui .

# Clean
clean:
	rm -f kafkaui
	rm -rf frontend/dist frontend/node_modules
```

**Step 7: Commit**

```bash
git add .gitignore go.mod go.sum config.example.yaml Makefile
git commit -m "Initialize Go module with dependencies and project scaffolding"
```

---

### Task 2: Config parsing with env var expansion

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write the failing test**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MinimalConfig(t *testing.T) {
	yaml := `
server:
  port: 9090
clusters:
  - name: test-cluster
    bootstrap-servers: localhost:9092
`
	path := writeTempFile(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if len(cfg.Clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(cfg.Clusters))
	}
	if cfg.Clusters[0].Name != "test-cluster" {
		t.Errorf("expected cluster name 'test-cluster', got %q", cfg.Clusters[0].Name)
	}
	if cfg.Clusters[0].BootstrapServers != "localhost:9092" {
		t.Errorf("expected bootstrap servers 'localhost:9092', got %q", cfg.Clusters[0].BootstrapServers)
	}
}

func TestLoad_DefaultPort(t *testing.T) {
	yaml := `
clusters:
  - name: test
    bootstrap-servers: localhost:9092
`
	path := writeTempFile(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
}

func TestLoad_EnvVarExpansion(t *testing.T) {
	t.Setenv("TEST_KAFKA_PASSWORD", "secret123")
	yaml := `
clusters:
  - name: test
    bootstrap-servers: localhost:9092
    sasl:
      mechanism: PLAIN
      username: admin
      password: ${TEST_KAFKA_PASSWORD}
`
	path := writeTempFile(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Clusters[0].SASL.Password != "secret123" {
		t.Errorf("expected password 'secret123', got %q", cfg.Clusters[0].SASL.Password)
	}
}

func TestLoad_FullClusterConfig(t *testing.T) {
	yaml := `
server:
  port: 8080
  base-path: /kafka
clusters:
  - name: production
    bootstrap-servers: kafka-1:9092,kafka-2:9092
    tls:
      enabled: true
      ca-file: /certs/ca.pem
    sasl:
      mechanism: SCRAM-SHA-512
      username: admin
      password: secret
    schema-registry:
      url: http://sr:8081
    kafka-connect:
      - name: main-connect
        url: http://connect:8083
    ksql:
      url: http://ksql:8088
`
	path := writeTempFile(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := cfg.Clusters[0]
	if c.TLS.Enabled != true {
		t.Error("expected TLS enabled")
	}
	if c.SchemaRegistry.URL != "http://sr:8081" {
		t.Errorf("expected schema registry URL, got %q", c.SchemaRegistry.URL)
	}
	if len(c.KafkaConnect) != 1 || c.KafkaConnect[0].Name != "main-connect" {
		t.Error("expected kafka connect config")
	}
	if c.KSQL.URL != "http://ksql:8088" {
		t.Errorf("expected ksql URL, got %q", c.KSQL.URL)
	}
	if cfg.Server.BasePath != "/kafka" {
		t.Errorf("expected base path '/kafka', got %q", cfg.Server.BasePath)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeTempFile(t, `{{{invalid yaml`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/config/... -v
```

Expected: FAIL — `Load` not defined.

**Step 3: Write implementation**

Create `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Auth    AuthConfig    `yaml:"auth"`
	Clusters []ClusterConfig `yaml:"clusters"`
}

type ServerConfig struct {
	Port     int    `yaml:"port"`
	BasePath string `yaml:"base-path"`
}

type AuthConfig struct {
	Enabled bool   `yaml:"enabled"`
	Type    string `yaml:"type"`
}

type ClusterConfig struct {
	Name             string              `yaml:"name"`
	BootstrapServers string              `yaml:"bootstrap-servers"`
	TLS              TLSConfig           `yaml:"tls"`
	SASL             SASLConfig          `yaml:"sasl"`
	SchemaRegistry   SchemaRegistryConfig `yaml:"schema-registry"`
	KafkaConnect     []KafkaConnectConfig `yaml:"kafka-connect"`
	KSQL             KSQLConfig          `yaml:"ksql"`
}

type TLSConfig struct {
	Enabled bool   `yaml:"enabled"`
	CAFile  string `yaml:"ca-file"`
}

type SASLConfig struct {
	Mechanism string `yaml:"mechanism"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
}

type SchemaRegistryConfig struct {
	URL string `yaml:"url"`
}

type KafkaConnectConfig struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

type KSQLConfig struct {
	URL string `yaml:"url"`
}

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	expanded := envVarPattern.ReplaceAllStringFunc(string(data), func(match string) string {
		varName := envVarPattern.FindStringSubmatch(match)[1]
		return os.Getenv(varName)
	})

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config YAML: %w", err)
	}

	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}

	return &cfg, nil
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/... -v
```

Expected: All PASS.

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "Add YAML config parsing with env var expansion"
```

---

### Task 3: Kafka admin client wrapper

**Files:**
- Create: `internal/kafka/client.go`
- Create: `internal/kafka/client_test.go`

**Step 1: Write the client interface and struct**

Create `internal/kafka/client.go`:

```go
package kafka

import (
	"context"
	"fmt"
	"strings"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

type Client struct {
	raw   *kgo.Client
	admin *kadm.Client
	name  string
}

type BrokerInfo struct {
	ID   int32  `json:"id"`
	Host string `json:"host"`
	Port int32  `json:"port"`
	Rack string `json:"rack,omitempty"`
}

type TopicInfo struct {
	Name       string           `json:"name"`
	Partitions int              `json:"partitions"`
	Replicas   int              `json:"replicas"`
	Internal   bool             `json:"internal"`
	Configs    map[string]string `json:"configs,omitempty"`
}

type TopicDetail struct {
	Name       string            `json:"name"`
	Partitions []PartitionInfo   `json:"partitions"`
	Configs    map[string]string `json:"configs"`
	Internal   bool              `json:"internal"`
}

type PartitionInfo struct {
	ID       int32   `json:"id"`
	Leader   int32   `json:"leader"`
	Replicas []int32 `json:"replicas"`
	ISR      []int32 `json:"isr"`
}

type CreateTopicRequest struct {
	Name       string `json:"name"`
	Partitions int32  `json:"partitions"`
	Replicas   int16  `json:"replicas"`
}

func NewClient(cfg config.ClusterConfig) (*Client, error) {
	seeds := strings.Split(cfg.BootstrapServers, ",")
	opts := []kgo.Opt{
		kgo.SeedBrokers(seeds...),
	}

	if cfg.SASL.Mechanism != "" {
		saslOpt, err := buildSASLOpt(cfg.SASL)
		if err != nil {
			return nil, fmt.Errorf("configuring SASL: %w", err)
		}
		opts = append(opts, saslOpt)
	}

	if cfg.TLS.Enabled {
		tlsOpt, err := buildTLSOpt(cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("configuring TLS: %w", err)
		}
		opts = append(opts, tlsOpt)
	}

	raw, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating kafka client: %w", err)
	}

	return &Client{
		raw:   raw,
		admin: kadm.NewClient(raw),
		name:  cfg.Name,
	}, nil
}

func (c *Client) Name() string {
	return c.name
}

func (c *Client) Close() {
	c.raw.Close()
}

func (c *Client) Brokers(ctx context.Context) ([]BrokerInfo, error) {
	meta, err := c.admin.BrokerMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching broker metadata: %w", err)
	}

	brokers := make([]BrokerInfo, 0, len(meta))
	for _, b := range meta {
		brokers = append(brokers, BrokerInfo{
			ID:   b.NodeID,
			Host: b.Host,
			Port: int32(b.Port),
			Rack: b.Rack,
		})
	}
	return brokers, nil
}

func (c *Client) Topics(ctx context.Context) ([]TopicInfo, error) {
	topics, err := c.admin.ListTopics(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing topics: %w", err)
	}

	result := make([]TopicInfo, 0, len(topics))
	for _, t := range topics.Sorted() {
		replicas := 0
		if len(t.Partitions) > 0 {
			replicas = len(t.Partitions[0].Replicas)
		}
		result = append(result, TopicInfo{
			Name:       t.Topic,
			Partitions: len(t.Partitions),
			Replicas:   replicas,
			Internal:   t.IsInternal,
		})
	}
	return result, nil
}

func (c *Client) TopicDetails(ctx context.Context, name string) (*TopicDetail, error) {
	topics, err := c.admin.ListTopics(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("describing topic: %w", err)
	}

	t, ok := topics[name]
	if !ok {
		return nil, fmt.Errorf("topic %q not found", name)
	}

	partitions := make([]PartitionInfo, 0, len(t.Partitions))
	for _, p := range t.Partitions.Sorted() {
		partitions = append(partitions, PartitionInfo{
			ID:       p.Partition,
			Leader:   p.Leader,
			Replicas: p.Replicas,
			ISR:      p.ISR,
		})
	}

	configs := make(map[string]string)
	rc, err := c.admin.DescribeTopicConfigs(ctx, name)
	if err == nil {
		for _, r := range rc {
			for _, cv := range r.Configs {
				if cv.Value != nil {
					configs[cv.Key] = *cv.Value
				}
			}
		}
	}

	return &TopicDetail{
		Name:       t.Topic,
		Partitions: partitions,
		Configs:    configs,
		Internal:   t.IsInternal,
	}, nil
}

func (c *Client) CreateTopic(ctx context.Context, req CreateTopicRequest) error {
	resp, err := c.admin.CreateTopics(ctx, int32(req.Partitions), req.Replicas, nil, req.Name)
	if err != nil {
		return fmt.Errorf("creating topic: %w", err)
	}
	for _, t := range resp.Sorted() {
		if t.Err != nil {
			return fmt.Errorf("creating topic %q: %w", t.Topic, t.Err)
		}
	}
	return nil
}

func (c *Client) DeleteTopic(ctx context.Context, name string) error {
	resp, err := c.admin.DeleteTopics(ctx, name)
	if err != nil {
		return fmt.Errorf("deleting topic: %w", err)
	}
	for _, t := range resp.Sorted() {
		if t.Err != nil {
			return fmt.Errorf("deleting topic %q: %w", t.Topic, t.Err)
		}
	}
	return nil
}
```

**Step 2: Create SASL and TLS helpers**

Create `internal/kafka/auth.go`:

```go
package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

func buildSASLOpt(cfg config.SASLConfig) (kgo.Opt, error) {
	switch cfg.Mechanism {
	case "PLAIN":
		return kgo.SASL(plain.Auth{
			User: cfg.Username,
			Pass: cfg.Password,
		}.AsMechanism()), nil
	case "SCRAM-SHA-256":
		return kgo.SASL(scram.Auth{
			User: cfg.Username,
			Pass: cfg.Password,
		}.AsSha256Mechanism()), nil
	case "SCRAM-SHA-512":
		return kgo.SASL(scram.Auth{
			User: cfg.Username,
			Pass: cfg.Password,
		}.AsSha512Mechanism()), nil
	default:
		return nil, fmt.Errorf("unsupported SASL mechanism: %s", cfg.Mechanism)
	}
}

func buildTLSOpt(cfg config.TLSConfig) (kgo.Opt, error) {
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if cfg.CAFile != "" {
		caCert, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("reading CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsCfg.RootCAs = pool
	}

	return kgo.DialTLSConfig(tlsCfg), nil
}
```

**Step 3: Write unit test for client construction (no Kafka needed)**

Create `internal/kafka/client_test.go`:

```go
package kafka

import (
	"testing"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

func TestNewClient_InvalidBroker(t *testing.T) {
	cfg := config.ClusterConfig{
		Name:             "test",
		BootstrapServers: "localhost:99999",
	}
	client, err := NewClient(cfg)
	// franz-go doesn't fail on construction with bad brokers,
	// it fails on first operation. So client should be non-nil.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Name() != "test" {
		t.Errorf("expected name 'test', got %q", client.Name())
	}
	client.Close()
}

func TestBuildSASLOpt_SupportedMechanisms(t *testing.T) {
	mechanisms := []string{"PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"}
	for _, m := range mechanisms {
		t.Run(m, func(t *testing.T) {
			cfg := config.SASLConfig{
				Mechanism: m,
				Username:  "user",
				Password:  "pass",
			}
			opt, err := buildSASLOpt(cfg)
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", m, err)
			}
			if opt == nil {
				t.Fatalf("expected non-nil opt for %s", m)
			}
		})
	}
}

func TestBuildSASLOpt_UnsupportedMechanism(t *testing.T) {
	cfg := config.SASLConfig{
		Mechanism: "UNSUPPORTED",
	}
	_, err := buildSASLOpt(cfg)
	if err == nil {
		t.Fatal("expected error for unsupported mechanism")
	}
}
```

**Step 4: Run tests**

```bash
go test ./internal/kafka/... -v
```

Expected: All PASS.

**Step 5: Commit**

```bash
git add internal/kafka/
git commit -m "Add Kafka admin client wrapper with SASL/TLS support"
```

---

### Task 4: Cluster registry (manages multiple Kafka clients)

**Files:**
- Create: `internal/kafka/registry.go`
- Create: `internal/kafka/registry_test.go`

**Step 1: Write the failing test**

Create `internal/kafka/registry_test.go`:

```go
package kafka

import (
	"testing"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

func TestRegistry_GetByName(t *testing.T) {
	cfg := &config.Config{
		Clusters: []config.ClusterConfig{
			{Name: "cluster-a", BootstrapServers: "localhost:9092"},
			{Name: "cluster-b", BootstrapServers: "localhost:9093"},
		},
	}

	reg, err := NewRegistry(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reg.Close()

	c, ok := reg.Get("cluster-a")
	if !ok {
		t.Fatal("expected to find cluster-a")
	}
	if c.Name() != "cluster-a" {
		t.Errorf("expected name 'cluster-a', got %q", c.Name())
	}

	_, ok = reg.Get("nonexistent")
	if ok {
		t.Fatal("expected not to find nonexistent cluster")
	}
}

func TestRegistry_List(t *testing.T) {
	cfg := &config.Config{
		Clusters: []config.ClusterConfig{
			{Name: "alpha", BootstrapServers: "localhost:9092"},
			{Name: "beta", BootstrapServers: "localhost:9093"},
		},
	}

	reg, err := NewRegistry(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reg.Close()

	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(list))
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/kafka/... -v -run TestRegistry
```

Expected: FAIL — `NewRegistry` not defined.

**Step 3: Write implementation**

Create `internal/kafka/registry.go`:

```go
package kafka

import (
	"fmt"

	"github.com/Smyrcu/KafkaUI/internal/config"
)

type ClusterInfo struct {
	Name             string `json:"name"`
	BootstrapServers string `json:"bootstrapServers"`
}

type Registry struct {
	clients map[string]*Client
	configs map[string]config.ClusterConfig
	order   []string
}

func NewRegistry(cfg *config.Config) (*Registry, error) {
	r := &Registry{
		clients: make(map[string]*Client, len(cfg.Clusters)),
		configs: make(map[string]config.ClusterConfig, len(cfg.Clusters)),
	}

	for _, cc := range cfg.Clusters {
		client, err := NewClient(cc)
		if err != nil {
			r.Close()
			return nil, fmt.Errorf("creating client for cluster %q: %w", cc.Name, err)
		}
		r.clients[cc.Name] = client
		r.configs[cc.Name] = cc
		r.order = append(r.order, cc.Name)
	}

	return r, nil
}

func (r *Registry) Get(name string) (*Client, bool) {
	c, ok := r.clients[name]
	return c, ok
}

func (r *Registry) GetConfig(name string) (config.ClusterConfig, bool) {
	c, ok := r.configs[name]
	return c, ok
}

func (r *Registry) List() []ClusterInfo {
	result := make([]ClusterInfo, 0, len(r.order))
	for _, name := range r.order {
		cfg := r.configs[name]
		result = append(result, ClusterInfo{
			Name:             cfg.Name,
			BootstrapServers: cfg.BootstrapServers,
		})
	}
	return result
}

func (r *Registry) Close() {
	for _, c := range r.clients {
		c.Close()
	}
}
```

**Step 4: Run tests**

```bash
go test ./internal/kafka/... -v
```

Expected: All PASS.

**Step 5: Commit**

```bash
git add internal/kafka/registry.go internal/kafka/registry_test.go
git commit -m "Add cluster registry for multi-cluster client management"
```

---

### Task 5: REST API — router, middleware, cluster handler

**Files:**
- Create: `internal/api/router.go`
- Create: `internal/api/middleware/logging.go`
- Create: `internal/api/handlers/cluster.go`
- Create: `internal/api/handlers/cluster_test.go`

**Step 1: Write logging middleware**

Create `internal/api/middleware/logging.go`:

```go
package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(ww, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.status,
				"duration", time.Since(start).String(),
			)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
```

**Step 2: Write cluster handler test**

Create `internal/api/handlers/cluster_test.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Smyrcu/KafkaUI/internal/config"
	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

func TestClusterHandler_List(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewClusterHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body []kafka.ClusterInfo
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(body))
	}
	if body[0].Name != "alpha" {
		t.Errorf("expected first cluster 'alpha', got %q", body[0].Name)
	}
}

func mustCreateRegistry(t *testing.T) *kafka.Registry {
	t.Helper()
	cfg := &config.Config{
		Clusters: []config.ClusterConfig{
			{Name: "alpha", BootstrapServers: "localhost:9092"},
			{Name: "beta", BootstrapServers: "localhost:9093"},
		},
	}
	reg, err := kafka.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}
	t.Cleanup(reg.Close)
	return reg
}
```

**Step 3: Run test to verify it fails**

```bash
go test ./internal/api/handlers/... -v
```

Expected: FAIL — `NewClusterHandler` not defined.

**Step 4: Write cluster handler**

Create `internal/api/handlers/cluster.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

type ClusterHandler struct {
	registry *kafka.Registry
}

func NewClusterHandler(reg *kafka.Registry) *ClusterHandler {
	return &ClusterHandler{registry: reg}
}

func (h *ClusterHandler) List(w http.ResponseWriter, r *http.Request) {
	clusters := h.registry.List()
	writeJSON(w, http.StatusOK, clusters)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
```

**Step 5: Run tests**

```bash
go test ./internal/api/handlers/... -v
```

Expected: All PASS.

**Step 6: Write the router**

Create `internal/api/router.go`:

```go
package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/Smyrcu/KafkaUI/internal/api/handlers"
	"github.com/Smyrcu/KafkaUI/internal/api/middleware"
	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

func NewRouter(registry *kafka.Registry, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Recoverer)
	r.Use(chimw.RequestID)
	r.Use(middleware.Logger(logger))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	clusterHandler := handlers.NewClusterHandler(registry)
	brokerHandler := handlers.NewBrokerHandler(registry)
	topicHandler := handlers.NewTopicHandler(registry)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/clusters", clusterHandler.List)

		r.Route("/clusters/{clusterName}", func(r chi.Router) {
			r.Get("/brokers", brokerHandler.List)

			r.Get("/topics", topicHandler.List)
			r.Post("/topics", topicHandler.Create)
			r.Get("/topics/{topicName}", topicHandler.Details)
			r.Delete("/topics/{topicName}", topicHandler.Delete)
		})
	})

	return r
}
```

**Step 7: Commit**

```bash
git add internal/api/
git commit -m "Add REST API router with cluster handler and logging middleware"
```

---

### Task 6: REST API — broker and topic handlers

**Files:**
- Create: `internal/api/handlers/broker.go`
- Create: `internal/api/handlers/broker_test.go`
- Create: `internal/api/handlers/topic.go`
- Create: `internal/api/handlers/topic_test.go`

**Step 1: Write broker handler test**

Create `internal/api/handlers/broker_test.go`:

```go
package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestBrokerHandler_List_ClusterNotFound(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewBrokerHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/nonexistent/brokers", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}
```

**Step 2: Write broker handler**

Create `internal/api/handlers/broker.go`:

```go
package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

type BrokerHandler struct {
	registry *kafka.Registry
}

func NewBrokerHandler(reg *kafka.Registry) *BrokerHandler {
	return &BrokerHandler{registry: reg}
}

func (h *BrokerHandler) List(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")
	client, ok := h.registry.Get(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	brokers, err := client.Brokers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, brokers)
}
```

**Step 3: Write topic handler test**

Create `internal/api/handlers/topic_test.go`:

```go
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestTopicHandler_List_ClusterNotFound(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewTopicHandler(reg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/nonexistent/topics", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestTopicHandler_Create_InvalidBody(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewTopicHandler(reg)

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/topics", body)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestTopicHandler_Create_MissingName(t *testing.T) {
	reg := mustCreateRegistry(t)
	h := NewTopicHandler(reg)

	payload := map[string]any{"partitions": 3, "replicas": 1}
	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/alpha/topics", bytes.NewBuffer(bodyBytes))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("clusterName", "alpha")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}
```

**Step 4: Write topic handler**

Create `internal/api/handlers/topic.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

type TopicHandler struct {
	registry *kafka.Registry
}

func NewTopicHandler(reg *kafka.Registry) *TopicHandler {
	return &TopicHandler{registry: reg}
}

func (h *TopicHandler) List(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")
	client, ok := h.registry.Get(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	topics, err := client.Topics(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, topics)
}

func (h *TopicHandler) Details(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")
	topicName := chi.URLParam(r, "topicName")

	client, ok := h.registry.Get(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	detail, err := client.TopicDetails(r.Context(), topicName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (h *TopicHandler) Create(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")
	client, ok := h.registry.Get(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	var req kafka.CreateTopicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "topic name is required")
		return
	}
	if req.Partitions <= 0 {
		req.Partitions = 1
	}
	if req.Replicas <= 0 {
		req.Replicas = 1
	}

	if err := client.CreateTopic(r.Context(), req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "created", "topic": req.Name})
}

func (h *TopicHandler) Delete(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")
	topicName := chi.URLParam(r, "topicName")

	client, ok := h.registry.Get(clusterName)
	if !ok {
		writeError(w, http.StatusNotFound, "cluster not found")
		return
	}

	if err := client.DeleteTopic(r.Context(), topicName); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "topic": topicName})
}
```

**Step 5: Run tests**

```bash
go test ./internal/api/... -v
```

Expected: All PASS.

**Step 6: Commit**

```bash
git add internal/api/
git commit -m "Add broker and topic REST handlers with validation"
```

---

### Task 7: Main entry point with embedded frontend support

**Files:**
- Create: `cmd/kafkaui/main.go`
- Create: `frontend/dist/.gitkeep` (placeholder for go:embed)

**Step 1: Create placeholder for frontend embed**

```bash
mkdir -p frontend/dist
touch frontend/dist/.gitkeep
```

**Step 2: Write main.go**

Create `cmd/kafkaui/main.go`:

```go
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Smyrcu/KafkaUI/internal/api"
	"github.com/Smyrcu/KafkaUI/internal/config"
	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

//go:embed all:../../frontend/dist
var frontendFS embed.FS

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	registry, err := kafka.NewRegistry(cfg)
	if err != nil {
		logger.Error("failed to create kafka registry", "error", err)
		os.Exit(1)
	}
	defer registry.Close()

	router := api.NewRouter(registry, logger)

	// Serve frontend static files
	frontendContent, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		logger.Error("failed to create frontend filesystem", "error", err)
		os.Exit(1)
	}
	fileServer := http.FileServer(http.FS(frontendContent))

	mux := http.NewServeMux()
	mux.Handle("/api/", router)
	mux.Handle("/ws/", router)
	mux.Handle("/", spaHandler(fileServer))

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("starting server", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}
	logger.Info("server stopped")
}

// spaHandler wraps a file server to serve index.html for any path
// that doesn't match a static file (SPA client-side routing support).
func spaHandler(fileServer http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fileServer.ServeHTTP(w, r)
	})
}
```

> **Note:** The `go:embed` path `all:../../frontend/dist` uses a relative path from `cmd/kafkaui/`. During development with `make dev`, the frontend runs on its own Vite dev server and the API uses CORS. For production builds, `frontend/dist` contains the built SPA.

**Step 3: Verify compilation**

```bash
go build ./cmd/kafkaui
```

Expected: Build succeeds (binary `kafkaui` created, though won't run without config).

**Step 4: Commit**

```bash
git add cmd/kafkaui/main.go frontend/dist/.gitkeep
git commit -m "Add main entry point with graceful shutdown and embedded frontend"
```

---

### Task 8: Frontend scaffolding — Vite + React + shadcn/ui + Tailwind

**Files:**
- Create: `frontend/` — full Vite React project with shadcn/ui

**Step 1: Scaffold Vite React TypeScript project**

```bash
cd /home/smyrcu/RiderProjects/KafkaUI
npm create vite@latest frontend -- --template react-ts
```

**Step 2: Install dependencies**

```bash
cd /home/smyrcu/RiderProjects/KafkaUI/frontend
npm install
npm install react-router-dom@7 @tanstack/react-query lucide-react clsx tailwind-merge class-variance-authority
npm install -D tailwindcss @tailwindcss/vite
```

**Step 3: Configure Tailwind — update `frontend/src/index.css`**

Replace entire `frontend/src/index.css` with:

```css
@import "tailwindcss";

@layer base {
  :root {
    --background: 0 0% 100%;
    --foreground: 240 10% 3.9%;
    --card: 0 0% 100%;
    --card-foreground: 240 10% 3.9%;
    --popover: 0 0% 100%;
    --popover-foreground: 240 10% 3.9%;
    --primary: 240 5.9% 10%;
    --primary-foreground: 0 0% 98%;
    --secondary: 240 4.8% 95.9%;
    --secondary-foreground: 240 5.9% 10%;
    --muted: 240 4.8% 95.9%;
    --muted-foreground: 240 3.8% 46.1%;
    --accent: 240 4.8% 95.9%;
    --accent-foreground: 240 5.9% 10%;
    --destructive: 0 84.2% 60.2%;
    --destructive-foreground: 0 0% 98%;
    --border: 240 5.9% 90%;
    --input: 240 5.9% 90%;
    --ring: 240 5.9% 10%;
    --radius: 0.5rem;
    --sidebar-background: 0 0% 98%;
    --sidebar-foreground: 240 5.3% 26.1%;
    --sidebar-primary: 240 5.9% 10%;
    --sidebar-primary-foreground: 0 0% 98%;
    --sidebar-accent: 240 4.8% 95.9%;
    --sidebar-accent-foreground: 240 5.9% 10%;
    --sidebar-border: 220 13% 91%;
    --sidebar-ring: 217.2 91.2% 59.8%;
  }

  .dark {
    --background: 240 10% 3.9%;
    --foreground: 0 0% 98%;
    --card: 240 10% 3.9%;
    --card-foreground: 0 0% 98%;
    --popover: 240 10% 3.9%;
    --popover-foreground: 0 0% 98%;
    --primary: 0 0% 98%;
    --primary-foreground: 240 5.9% 10%;
    --secondary: 240 3.7% 15.9%;
    --secondary-foreground: 0 0% 98%;
    --muted: 240 3.7% 15.9%;
    --muted-foreground: 240 5% 64.9%;
    --accent: 240 3.7% 15.9%;
    --accent-foreground: 0 0% 98%;
    --destructive: 0 62.8% 30.6%;
    --destructive-foreground: 0 0% 98%;
    --border: 240 3.7% 15.9%;
    --input: 240 3.7% 15.9%;
    --ring: 240 4.9% 83.9%;
    --sidebar-background: 240 5.9% 10%;
    --sidebar-foreground: 240 4.8% 95.9%;
    --sidebar-primary: 224.3 76.3% 48%;
    --sidebar-primary-foreground: 0 0% 100%;
    --sidebar-accent: 240 3.7% 15.9%;
    --sidebar-accent-foreground: 240 4.8% 95.9%;
    --sidebar-border: 240 3.7% 15.9%;
    --sidebar-ring: 217.2 91.2% 59.8%;
  }
}

@layer base {
  * {
    @apply border-border;
  }
  body {
    @apply bg-background text-foreground;
  }
}
```

**Step 4: Add Tailwind Vite plugin — update `frontend/vite.config.ts`**

```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/ws': {
        target: 'http://localhost:8080',
        ws: true,
      },
    },
  },
})
```

**Step 5: Create utility file `frontend/src/lib/utils.ts`**

```typescript
import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
```

**Step 6: Create API client `frontend/src/lib/api.ts`**

```typescript
const API_BASE = '/api/v1';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: {
      'Content-Type': 'application/json',
    },
    ...options,
  });

  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(error.error || res.statusText);
  }

  return res.json();
}

export interface ClusterInfo {
  name: string;
  bootstrapServers: string;
}

export interface BrokerInfo {
  id: number;
  host: string;
  port: number;
  rack?: string;
}

export interface TopicInfo {
  name: string;
  partitions: number;
  replicas: number;
  internal: boolean;
}

export interface TopicDetail {
  name: string;
  partitions: PartitionInfo[];
  configs: Record<string, string>;
  internal: boolean;
}

export interface PartitionInfo {
  id: number;
  leader: number;
  replicas: number[];
  isr: number[];
}

export interface CreateTopicRequest {
  name: string;
  partitions: number;
  replicas: number;
}

export const api = {
  clusters: {
    list: () => request<ClusterInfo[]>('/clusters'),
  },
  brokers: {
    list: (cluster: string) => request<BrokerInfo[]>(`/clusters/${cluster}/brokers`),
  },
  topics: {
    list: (cluster: string) => request<TopicInfo[]>(`/clusters/${cluster}/topics`),
    details: (cluster: string, topic: string) =>
      request<TopicDetail>(`/clusters/${cluster}/topics/${topic}`),
    create: (cluster: string, data: CreateTopicRequest) =>
      request(`/clusters/${cluster}/topics`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    delete: (cluster: string, topic: string) =>
      request(`/clusters/${cluster}/topics/${topic}`, {
        method: 'DELETE',
      }),
  },
};
```

**Step 7: Add `frontend/tsconfig.json` path alias**

Update `frontend/tsconfig.json` — add to `compilerOptions`:

```json
{
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {
      "@/*": ["./src/*"]
    }
  }
}
```

(Merge with existing content — keep all existing options.)

**Step 8: Verify frontend builds**

```bash
cd /home/smyrcu/RiderProjects/KafkaUI/frontend
npm run build
```

Expected: Build succeeds, `dist/` created.

**Step 9: Commit**

```bash
cd /home/smyrcu/RiderProjects/KafkaUI
git add frontend/ -f
git commit -m "Scaffold React frontend with Vite, shadcn/ui, Tailwind, and API client"
```

---

### Task 9: shadcn/ui components — install base components

**Files:**
- Create: `frontend/src/components/ui/button.tsx`
- Create: `frontend/src/components/ui/card.tsx`
- Create: `frontend/src/components/ui/table.tsx`
- Create: `frontend/src/components/ui/badge.tsx`
- Create: `frontend/src/components/ui/input.tsx`
- Create: `frontend/src/components/ui/dialog.tsx`
- Create: `frontend/src/components/ui/label.tsx`
- Create: `frontend/src/components/ui/separator.tsx`
- Create: `frontend/src/components/ui/sidebar.tsx`
- Create: `frontend/components.json`

**Step 1: Initialize shadcn/ui**

```bash
cd /home/smyrcu/RiderProjects/KafkaUI/frontend
npx shadcn@latest init -d
```

Answer prompts: New York style, Zinc color, CSS variables = yes.

**Step 2: Install required shadcn components**

```bash
cd /home/smyrcu/RiderProjects/KafkaUI/frontend
npx shadcn@latest add button card table badge input dialog label separator sidebar tooltip sheet
```

**Step 3: Verify build still works**

```bash
cd /home/smyrcu/RiderProjects/KafkaUI/frontend
npm run build
```

Expected: Build succeeds.

**Step 4: Commit**

```bash
cd /home/smyrcu/RiderProjects/KafkaUI
git add frontend/
git commit -m "Install shadcn/ui base components"
```

---

### Task 10: Frontend layout — Sidebar + TopBar

**Files:**
- Create: `frontend/src/components/layout/AppSidebar.tsx`
- Create: `frontend/src/components/layout/TopBar.tsx`
- Create: `frontend/src/components/layout/Layout.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/main.tsx`

**Step 1: Create AppSidebar**

Create `frontend/src/components/layout/AppSidebar.tsx`:

```tsx
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar";
import { Link, useParams } from "react-router-dom";
import {
  Database,
  Server,
  FileText,
  Users,
  Shield,
  PlugZap,
  Terminal,
  BookOpen,
} from "lucide-react";

const navItems = [
  { label: "Brokers", icon: Server, path: "brokers" },
  { label: "Topics", icon: FileText, path: "topics" },
  { label: "Consumer Groups", icon: Users, path: "consumer-groups" },
  { label: "Schema Registry", icon: BookOpen, path: "schemas" },
  { label: "Kafka Connect", icon: PlugZap, path: "connect" },
  { label: "KSQL", icon: Terminal, path: "ksql" },
  { label: "ACL", icon: Shield, path: "acl" },
];

export function AppSidebar() {
  const { clusterName } = useParams();

  return (
    <Sidebar>
      <SidebarHeader>
        <Link to="/" className="flex items-center gap-2 px-2 py-1">
          <Database className="h-6 w-6" />
          <span className="text-lg font-bold">KafkaUI</span>
        </Link>
      </SidebarHeader>
      <SidebarContent>
        {clusterName && (
          <SidebarGroup>
            <SidebarGroupLabel>{clusterName}</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {navItems.map((item) => (
                  <SidebarMenuItem key={item.path}>
                    <SidebarMenuButton asChild>
                      <Link to={`/clusters/${clusterName}/${item.path}`}>
                        <item.icon className="h-4 w-4" />
                        <span>{item.label}</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        )}
      </SidebarContent>
    </Sidebar>
  );
}
```

**Step 2: Create Layout**

Create `frontend/src/components/layout/Layout.tsx`:

```tsx
import { SidebarProvider, SidebarTrigger } from "@/components/ui/sidebar";
import { AppSidebar } from "./AppSidebar";
import { Outlet } from "react-router-dom";
import { Separator } from "@/components/ui/separator";

export function Layout() {
  return (
    <SidebarProvider>
      <AppSidebar />
      <main className="flex-1 flex flex-col min-h-screen">
        <header className="flex h-14 items-center gap-2 border-b px-4">
          <SidebarTrigger />
          <Separator orientation="vertical" className="h-6" />
          <h1 className="text-sm font-medium">Kafka UI</h1>
        </header>
        <div className="flex-1 p-6">
          <Outlet />
        </div>
      </main>
    </SidebarProvider>
  );
}
```

**Step 3: Update App.tsx with routing**

Replace `frontend/src/App.tsx`:

```tsx
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Layout } from "@/components/layout/Layout";
import { ClustersPage } from "@/pages/ClustersPage";
import { BrokersPage } from "@/pages/BrokersPage";
import { TopicsPage } from "@/pages/TopicsPage";
import { TopicDetailPage } from "@/pages/TopicDetailPage";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
});

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<ClustersPage />} />
            <Route path="/clusters/:clusterName/brokers" element={<BrokersPage />} />
            <Route path="/clusters/:clusterName/topics" element={<TopicsPage />} />
            <Route path="/clusters/:clusterName/topics/:topicName" element={<TopicDetailPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
```

**Step 4: Update main.tsx**

Replace `frontend/src/main.tsx`:

```tsx
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import App from "./App";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>
);
```

**Step 5: Commit**

```bash
cd /home/smyrcu/RiderProjects/KafkaUI
git add frontend/src/
git commit -m "Add sidebar layout with navigation and React Router setup"
```

---

### Task 11: Frontend pages — Clusters, Brokers, Topics

**Files:**
- Create: `frontend/src/pages/ClustersPage.tsx`
- Create: `frontend/src/pages/BrokersPage.tsx`
- Create: `frontend/src/pages/TopicsPage.tsx`
- Create: `frontend/src/pages/TopicDetailPage.tsx`

**Step 1: Create ClustersPage**

Create `frontend/src/pages/ClustersPage.tsx`:

```tsx
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { Link } from "react-router-dom";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Database } from "lucide-react";

export function ClustersPage() {
  const { data: clusters, isLoading, error } = useQuery({
    queryKey: ["clusters"],
    queryFn: api.clusters.list,
  });

  if (isLoading) return <div className="text-muted-foreground">Loading clusters...</div>;
  if (error) return <div className="text-destructive">Error: {(error as Error).message}</div>;

  return (
    <div>
      <h2 className="text-2xl font-bold mb-6">Clusters</h2>
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {clusters?.map((cluster) => (
          <Link key={cluster.name} to={`/clusters/${cluster.name}/brokers`}>
            <Card className="hover:border-primary transition-colors cursor-pointer">
              <CardHeader className="flex flex-row items-center gap-3">
                <Database className="h-5 w-5 text-muted-foreground" />
                <CardTitle className="text-lg">{cluster.name}</CardTitle>
              </CardHeader>
              <CardContent>
                <Badge variant="secondary">{cluster.bootstrapServers}</Badge>
              </CardContent>
            </Card>
          </Link>
        ))}
      </div>
    </div>
  );
}
```

**Step 2: Create BrokersPage**

Create `frontend/src/pages/BrokersPage.tsx`:

```tsx
import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { api } from "@/lib/api";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";

export function BrokersPage() {
  const { clusterName } = useParams<{ clusterName: string }>();
  const { data: brokers, isLoading, error } = useQuery({
    queryKey: ["brokers", clusterName],
    queryFn: () => api.brokers.list(clusterName!),
    enabled: !!clusterName,
  });

  if (isLoading) return <div className="text-muted-foreground">Loading brokers...</div>;
  if (error) return <div className="text-destructive">Error: {(error as Error).message}</div>;

  return (
    <div>
      <h2 className="text-2xl font-bold mb-6">Brokers</h2>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>ID</TableHead>
            <TableHead>Host</TableHead>
            <TableHead>Port</TableHead>
            <TableHead>Rack</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {brokers?.map((broker) => (
            <TableRow key={broker.id}>
              <TableCell>
                <Badge variant="outline">{broker.id}</Badge>
              </TableCell>
              <TableCell>{broker.host}</TableCell>
              <TableCell>{broker.port}</TableCell>
              <TableCell>{broker.rack || "—"}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
```

**Step 3: Create TopicsPage**

Create `frontend/src/pages/TopicsPage.tsx`:

```tsx
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams, Link } from "react-router-dom";
import { api, CreateTopicRequest } from "@/lib/api";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogFooter,
} from "@/components/ui/dialog";
import { Plus, Trash2 } from "lucide-react";

export function TopicsPage() {
  const { clusterName } = useParams<{ clusterName: string }>();
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [newTopic, setNewTopic] = useState<CreateTopicRequest>({
    name: "",
    partitions: 1,
    replicas: 1,
  });

  const { data: topics, isLoading, error } = useQuery({
    queryKey: ["topics", clusterName],
    queryFn: () => api.topics.list(clusterName!),
    enabled: !!clusterName,
  });

  const createMutation = useMutation({
    mutationFn: (data: CreateTopicRequest) => api.topics.create(clusterName!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["topics", clusterName] });
      setOpen(false);
      setNewTopic({ name: "", partitions: 1, replicas: 1 });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (topicName: string) => api.topics.delete(clusterName!, topicName),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["topics", clusterName] });
    },
  });

  const filteredTopics = topics?.filter((t) =>
    t.name.toLowerCase().includes(search.toLowerCase())
  );

  if (isLoading) return <div className="text-muted-foreground">Loading topics...</div>;
  if (error) return <div className="text-destructive">Error: {(error as Error).message}</div>;

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-bold">Topics</h2>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="h-4 w-4 mr-2" />
              Create Topic
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Create Topic</DialogTitle>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="name">Name</Label>
                <Input
                  id="name"
                  value={newTopic.name}
                  onChange={(e) => setNewTopic({ ...newTopic, name: e.target.value })}
                  placeholder="my-topic"
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div className="grid gap-2">
                  <Label htmlFor="partitions">Partitions</Label>
                  <Input
                    id="partitions"
                    type="number"
                    min={1}
                    value={newTopic.partitions}
                    onChange={(e) =>
                      setNewTopic({ ...newTopic, partitions: parseInt(e.target.value) || 1 })
                    }
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="replicas">Replicas</Label>
                  <Input
                    id="replicas"
                    type="number"
                    min={1}
                    value={newTopic.replicas}
                    onChange={(e) =>
                      setNewTopic({ ...newTopic, replicas: parseInt(e.target.value) || 1 })
                    }
                  />
                </div>
              </div>
            </div>
            <DialogFooter>
              <Button
                onClick={() => createMutation.mutate(newTopic)}
                disabled={!newTopic.name || createMutation.isPending}
              >
                {createMutation.isPending ? "Creating..." : "Create"}
              </Button>
            </DialogFooter>
            {createMutation.isError && (
              <p className="text-sm text-destructive mt-2">
                {(createMutation.error as Error).message}
              </p>
            )}
          </DialogContent>
        </Dialog>
      </div>

      <div className="mb-4">
        <Input
          placeholder="Search topics..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-sm"
        />
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Partitions</TableHead>
            <TableHead>Replicas</TableHead>
            <TableHead>Internal</TableHead>
            <TableHead className="w-[80px]">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {filteredTopics?.map((topic) => (
            <TableRow key={topic.name}>
              <TableCell>
                <Link
                  to={`/clusters/${clusterName}/topics/${topic.name}`}
                  className="text-primary hover:underline font-medium"
                >
                  {topic.name}
                </Link>
              </TableCell>
              <TableCell>{topic.partitions}</TableCell>
              <TableCell>{topic.replicas}</TableCell>
              <TableCell>
                {topic.internal && <Badge variant="secondary">internal</Badge>}
              </TableCell>
              <TableCell>
                {!topic.internal && (
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => {
                      if (confirm(`Delete topic "${topic.name}"?`)) {
                        deleteMutation.mutate(topic.name);
                      }
                    }}
                  >
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
```

**Step 4: Create TopicDetailPage**

Create `frontend/src/pages/TopicDetailPage.tsx`:

```tsx
import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { api } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";

export function TopicDetailPage() {
  const { clusterName, topicName } = useParams<{
    clusterName: string;
    topicName: string;
  }>();

  const { data: topic, isLoading, error } = useQuery({
    queryKey: ["topic", clusterName, topicName],
    queryFn: () => api.topics.details(clusterName!, topicName!),
    enabled: !!clusterName && !!topicName,
  });

  if (isLoading) return <div className="text-muted-foreground">Loading topic details...</div>;
  if (error) return <div className="text-destructive">Error: {(error as Error).message}</div>;
  if (!topic) return null;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <h2 className="text-2xl font-bold">{topic.name}</h2>
        {topic.internal && <Badge variant="secondary">internal</Badge>}
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Partitions</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Partition</TableHead>
                  <TableHead>Leader</TableHead>
                  <TableHead>Replicas</TableHead>
                  <TableHead>ISR</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {topic.partitions.map((p) => (
                  <TableRow key={p.id}>
                    <TableCell>
                      <Badge variant="outline">{p.id}</Badge>
                    </TableCell>
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
          <CardHeader>
            <CardTitle>Configuration</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Key</TableHead>
                  <TableHead>Value</TableHead>
                </TableRow>
              </TableHeader>
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

**Step 5: Verify build**

```bash
cd /home/smyrcu/RiderProjects/KafkaUI/frontend
npm run build
```

Expected: Build succeeds.

**Step 6: Commit**

```bash
cd /home/smyrcu/RiderProjects/KafkaUI
git add frontend/src/pages/
git commit -m "Add cluster, broker, topic, and topic detail pages"
```

---

### Task 12: Dockerfile and final integration test

**Files:**
- Create: `Dockerfile`

**Step 1: Create Dockerfile**

Create `Dockerfile` at project root:

```dockerfile
# Stage 1: Frontend build
FROM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# Stage 2: Go build
FROM golang:1.23-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 go build -o kafkaui ./cmd/kafkaui

# Stage 3: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=backend /app/kafkaui /usr/local/bin/
EXPOSE 8080
ENTRYPOINT ["kafkaui"]
CMD ["--config", "/etc/kafkaui/config.yaml"]
```

**Step 2: Verify all Go tests pass**

```bash
cd /home/smyrcu/RiderProjects/KafkaUI
go test ./... -v
```

Expected: All PASS.

**Step 3: Verify frontend builds**

```bash
cd /home/smyrcu/RiderProjects/KafkaUI/frontend
npm run build
```

Expected: Build succeeds.

**Step 4: Commit**

```bash
cd /home/smyrcu/RiderProjects/KafkaUI
git add Dockerfile
git commit -m "Add multi-stage Dockerfile for production builds"
```

---

## Summary

After completing all 12 tasks, you will have:

- **Go backend** with config parsing (YAML + env vars), Kafka admin client (franz-go), cluster registry, REST API (chi) for clusters/brokers/topics CRUD
- **React frontend** with shadcn/ui, Tailwind CSS, React Router, TanStack Query — pages for cluster list, broker list, topic list with search/create/delete, and topic details
- **Embedded frontend** via `go:embed` for single-binary deployment
- **Dockerfile** for containerized deployment
- **Makefile** for dev/build/test/docker workflows
- **Full test coverage** for config parsing, Kafka client, and HTTP handlers

The app is ready to connect to a real Kafka cluster via `config.yaml` and browse clusters, brokers, and topics through the web UI.
