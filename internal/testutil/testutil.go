package testutil

import (
	"log/slog"
	"testing"

	"github.com/Smyrcu/KafkaUI/internal/config"
	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

// MustCreateRegistry creates a Registry with two test clusters ("alpha" and "beta")
// using ports unlikely to conflict with a running Kafka instance.
// The registry is automatically closed when the test completes.
func MustCreateRegistry(t *testing.T) *kafka.Registry {
	t.Helper()
	cfg := &config.Config{
		Clusters: []config.ClusterConfig{
			{Name: "alpha", BootstrapServers: "localhost:19092"},
			{Name: "beta", BootstrapServers: "localhost:19093"},
		},
	}
	reg, err := kafka.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}
	t.Cleanup(reg.Close)
	return reg
}

// TestLogger returns a logger that discards all output, suitable for use in tests.
func TestLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
