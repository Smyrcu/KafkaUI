package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig    `yaml:"server"`
	Auth     AuthConfig      `yaml:"auth"`
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
	Name             string               `yaml:"name"`
	BootstrapServers string               `yaml:"bootstrap-servers"`
	TLS              TLSConfig            `yaml:"tls"`
	SASL             SASLConfig           `yaml:"sasl"`
	SchemaRegistry   SchemaRegistryConfig `yaml:"schema-registry"`
	KafkaConnect     []KafkaConnectConfig `yaml:"kafka-connect"`
	KSQL             KSQLConfig           `yaml:"ksql"`
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
