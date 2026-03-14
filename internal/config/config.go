package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server      ServerConfig    `yaml:"server"`
	Auth        AuthConfig      `yaml:"auth"`
	Clusters    []ClusterConfig `yaml:"clusters"`
	DataMasking DataMaskingConfig `yaml:"data-masking"`
}

type DataMaskingConfig struct {
	Rules []MaskingRule `yaml:"rules"`
}

type MaskingRule struct {
	TopicPattern string        `yaml:"topic-pattern"`
	Fields       []MaskingField `yaml:"fields"`
}

type MaskingField struct {
	Path string `yaml:"path"`
	Type string `yaml:"type"` // mask, hide, hash
}

type ServerConfig struct {
	Port     int    `yaml:"port"`
	BasePath string `yaml:"base-path"`
	Debug    bool   `yaml:"debug"`
}

type AuthConfig struct {
	Enabled        bool                 `yaml:"enabled"`
	Types          []string             `yaml:"types"`
	DefaultRole    string               `yaml:"default-role"`
	OIDC           OIDCConfig           `yaml:"oidc"`
	OAuth2         OAuth2Config         `yaml:"oauth2"`
	Basic          BasicAuthConfig      `yaml:"basic"`
	Session        SessionConfig        `yaml:"session"`
	RBAC           RBACConfig           `yaml:"rbac"`
	AutoAssignment []AutoAssignmentRule `yaml:"auto-assignment"`
	Storage        StorageConfig        `yaml:"storage"`
}

type BasicAuthConfig struct {
	Users     []BasicUser     `yaml:"users"`
	RateLimit RateLimitConfig `yaml:"rate-limit"`
}

type RateLimitConfig struct {
	MaxAttempts   int `yaml:"max-attempts"`
	WindowSeconds int `yaml:"window-seconds"`
}

type BasicUser struct {
	Username string   `yaml:"username"`
	Password string   `yaml:"password"` // bcrypt hash
	Roles    []string `yaml:"roles"`
}

type OIDCConfig struct {
	RedirectURL string         `yaml:"redirect-url"`
	Providers   []OIDCProvider `yaml:"providers"`
}

type OIDCProvider struct {
	Name         string   `yaml:"name"`
	DisplayName  string   `yaml:"display-name"`
	Issuer       string   `yaml:"issuer"`
	ClientID     string   `yaml:"client-id"`
	ClientSecret string   `yaml:"client-secret"`
	Scopes       []string `yaml:"scopes"`
}

type SessionConfig struct {
	Secret string `yaml:"secret"`
	MaxAge int    `yaml:"max-age"`
}

type OAuth2Config struct {
	RedirectURL string           `yaml:"redirect-url"`
	Providers   []OAuth2Provider `yaml:"providers"`
}

type OAuth2Provider struct {
	Name         string   `yaml:"name"`
	DisplayName  string   `yaml:"display-name"`
	ClientID     string   `yaml:"client-id"`
	ClientSecret string   `yaml:"client-secret"`
	Scopes       []string `yaml:"scopes"`
}

type RBACConfig struct {
	RoleGroups map[string][]string `yaml:"role-groups"`
	Rules      []RBACRule          `yaml:"rules"`
}

type RBACRule struct {
	Role     string   `yaml:"role"`
	Clusters []string `yaml:"clusters"`
	Actions  []string `yaml:"actions"`
}

type AutoAssignmentRule struct {
	Role  string              `yaml:"role"`
	Match AutoAssignmentMatch `yaml:"match"`
}

type AutoAssignmentMatch struct {
	Authenticated bool     `yaml:"authenticated"`
	Emails        []string `yaml:"emails"`
	EmailDomains  []string `yaml:"email-domains"`
	GitHubOrgs    []string `yaml:"github-orgs"`
	GitHubTeams   []string `yaml:"github-teams"`
	GitLabGroups  []string `yaml:"gitlab-groups"`
}

type StorageConfig struct {
	Path string `yaml:"path"`
}

type ClusterConfig struct {
	Name             string               `yaml:"name"`
	BootstrapServers string               `yaml:"bootstrap-servers"`
	TLS              TLSConfig            `yaml:"tls"`
	SASL             SASLConfig           `yaml:"sasl"`
	SchemaRegistry   SchemaRegistryConfig `yaml:"schema-registry"`
	KafkaConnect     []KafkaConnectConfig `yaml:"kafka-connect"`
	KSQL             KSQLConfig           `yaml:"ksql"`
	Metrics          MetricsConfig        `yaml:"metrics"`
}

type MetricsConfig struct {
	URL string `yaml:"url"`
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
