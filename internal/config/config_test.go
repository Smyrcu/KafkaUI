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
