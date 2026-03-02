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

func TestLoad_MultipleClusters(t *testing.T) {
	yaml := `
clusters:
  - name: cluster-a
    bootstrap-servers: host-a:9092
  - name: cluster-b
    bootstrap-servers: host-b:9093
  - name: cluster-c
    bootstrap-servers: host-c:9094
`
	path := writeTempFile(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Clusters) != 3 {
		t.Fatalf("expected 3 clusters, got %d", len(cfg.Clusters))
	}
	if cfg.Clusters[0].Name != "cluster-a" {
		t.Errorf("expected first cluster 'cluster-a', got %q", cfg.Clusters[0].Name)
	}
	if cfg.Clusters[2].Name != "cluster-c" {
		t.Errorf("expected third cluster 'cluster-c', got %q", cfg.Clusters[2].Name)
	}
}

func TestLoad_EmptyClusters(t *testing.T) {
	yaml := `
server:
  port: 8080
clusters: []
`
	path := writeTempFile(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Clusters) != 0 {
		t.Fatalf("expected 0 clusters, got %d", len(cfg.Clusters))
	}
}

func TestLoad_AuthConfig(t *testing.T) {
	yaml := `
auth:
  enabled: true
  type: oidc
clusters:
  - name: test
    bootstrap-servers: localhost:9092
`
	path := writeTempFile(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Auth.Enabled {
		t.Error("expected auth enabled")
	}
	if cfg.Auth.Type != "oidc" {
		t.Errorf("expected auth type 'oidc', got %q", cfg.Auth.Type)
	}
}

func TestLoad_TLSConfig(t *testing.T) {
	yaml := `
clusters:
  - name: secure
    bootstrap-servers: kafka:9092
    tls:
      enabled: true
      ca-file: /path/to/ca.pem
`
	path := writeTempFile(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Clusters[0].TLS.Enabled {
		t.Error("expected TLS enabled")
	}
	if cfg.Clusters[0].TLS.CAFile != "/path/to/ca.pem" {
		t.Errorf("expected CA file path, got %q", cfg.Clusters[0].TLS.CAFile)
	}
}

func TestLoad_SASLConfig(t *testing.T) {
	yaml := `
clusters:
  - name: sasl-test
    bootstrap-servers: kafka:9092
    sasl:
      mechanism: SCRAM-SHA-256
      username: myuser
      password: mypass
`
	path := writeTempFile(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := cfg.Clusters[0].SASL
	if s.Mechanism != "SCRAM-SHA-256" {
		t.Errorf("expected mechanism 'SCRAM-SHA-256', got %q", s.Mechanism)
	}
	if s.Username != "myuser" {
		t.Errorf("expected username 'myuser', got %q", s.Username)
	}
	if s.Password != "mypass" {
		t.Errorf("expected password 'mypass', got %q", s.Password)
	}
}

func TestLoad_MultipleEnvVars(t *testing.T) {
	t.Setenv("TEST_USER", "admin")
	t.Setenv("TEST_PASS", "secret")
	yaml := `
clusters:
  - name: test
    bootstrap-servers: localhost:9092
    sasl:
      mechanism: PLAIN
      username: ${TEST_USER}
      password: ${TEST_PASS}
`
	path := writeTempFile(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Clusters[0].SASL.Username != "admin" {
		t.Errorf("expected username 'admin', got %q", cfg.Clusters[0].SASL.Username)
	}
	if cfg.Clusters[0].SASL.Password != "secret" {
		t.Errorf("expected password 'secret', got %q", cfg.Clusters[0].SASL.Password)
	}
}

func TestLoad_ServerBasePath(t *testing.T) {
	yaml := `
server:
  port: 3000
  base-path: /kafka-ui
clusters:
  - name: test
    bootstrap-servers: localhost:9092
`
	path := writeTempFile(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.BasePath != "/kafka-ui" {
		t.Errorf("expected base-path '/kafka-ui', got %q", cfg.Server.BasePath)
	}
	if cfg.Server.Port != 3000 {
		t.Errorf("expected port 3000, got %d", cfg.Server.Port)
	}
}

func TestLoad_BasicAuthConfig(t *testing.T) {
	yaml := `
auth:
  enabled: true
  type: basic
  basic:
    users:
      - username: admin
        password: "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ12"
        roles: [admin]
      - username: viewer
        password: "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ12"
        roles: [viewer]
  rbac:
    - role: admin
      clusters: ["*"]
      actions: ["*"]
clusters:
  - name: test
    bootstrap-servers: localhost:9092
`
	path := writeTempFile(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Auth.Enabled {
		t.Error("expected auth enabled")
	}
	if cfg.Auth.Type != "basic" {
		t.Errorf("expected auth type 'basic', got %q", cfg.Auth.Type)
	}
	if len(cfg.Auth.Basic.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(cfg.Auth.Basic.Users))
	}
	if cfg.Auth.Basic.Users[0].Username != "admin" {
		t.Errorf("expected username 'admin', got %q", cfg.Auth.Basic.Users[0].Username)
	}
	if len(cfg.Auth.Basic.Users[0].Roles) != 1 || cfg.Auth.Basic.Users[0].Roles[0] != "admin" {
		t.Errorf("expected roles [admin], got %v", cfg.Auth.Basic.Users[0].Roles)
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
