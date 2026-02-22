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
			opt, err := BuildSASLOpt(cfg)
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
	_, err := BuildSASLOpt(cfg)
	if err == nil {
		t.Fatal("expected error for unsupported mechanism")
	}
}

func TestNewClient_WithSASL(t *testing.T) {
	cfg := config.ClusterConfig{
		Name:             "sasl-test",
		BootstrapServers: "localhost:9092",
		SASL: config.SASLConfig{
			Mechanism: "PLAIN",
			Username:  "user",
			Password:  "pass",
		},
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer client.Close()
	if client.Name() != "sasl-test" {
		t.Errorf("expected name 'sasl-test', got %q", client.Name())
	}
}

func TestNewClient_WithTLS(t *testing.T) {
	cfg := config.ClusterConfig{
		Name:             "tls-test",
		BootstrapServers: "localhost:9092",
		TLS: config.TLSConfig{
			Enabled: true,
		},
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer client.Close()
	if client.Name() != "tls-test" {
		t.Errorf("expected name 'tls-test', got %q", client.Name())
	}
}

func TestNewClient_ConfigAccessor(t *testing.T) {
	cfg := config.ClusterConfig{
		Name:             "config-test",
		BootstrapServers: "host1:9092,host2:9093",
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer client.Close()
	if client.Config().BootstrapServers != "host1:9092,host2:9093" {
		t.Errorf("expected bootstrap servers, got %q", client.Config().BootstrapServers)
	}
}
