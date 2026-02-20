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

func BuildSASLOpt(cfg config.SASLConfig) (kgo.Opt, error) {
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

func BuildTLSOpt(cfg config.TLSConfig) (kgo.Opt, error) {
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
