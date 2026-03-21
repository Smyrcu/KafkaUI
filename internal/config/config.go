package config

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"time"

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
	Port        int      `yaml:"port"`
	BasePath    string   `yaml:"base-path"`
	Debug       bool     `yaml:"debug"`
	TrustProxy  bool     `yaml:"trust-proxy"`
	CORSOrigins []string `yaml:"cors-origins"`
}

type AuthConfig struct {
	Enabled        bool                 `yaml:"enabled"`
	Types          []string             `yaml:"types"`
	DefaultRole    string               `yaml:"default-role"`
	OIDC           OIDCConfig           `yaml:"oidc"`
	OAuth2         OAuth2Config         `yaml:"oauth2"`
	Basic          BasicAuthConfig      `yaml:"basic"`
	LDAP           LDAPConfig           `yaml:"ldap"`
	Session        SessionConfig        `yaml:"session"`
	RBAC           RBACConfig           `yaml:"rbac"`
	AutoAssignment []AutoAssignmentRule `yaml:"auto-assignment"`
	Storage        StorageConfig        `yaml:"storage"`
}

type LDAPConfig struct {
	URL               string `yaml:"url"`
	StartTLS          bool   `yaml:"start-tls"`
	ConnectionTimeout string `yaml:"connection-timeout"`
	BindDN            string `yaml:"bind-dn"`
	BindPassword      string `yaml:"bind-password"`
	SearchBase        string `yaml:"search-base"`
	SearchFilter      string `yaml:"search-filter"`
	EmailAttribute    string `yaml:"email-attribute"`
	NameAttribute     string `yaml:"name-attribute"`
	GroupAttribute    string `yaml:"group-attribute"`
	GroupSearchBase   string `yaml:"group-search-base"`
	GroupSearchFilter string `yaml:"group-search-filter"`
}

// ConnectionTimeoutDuration returns the parsed timeout or 10s default.
func (c LDAPConfig) ConnectionTimeoutDuration() time.Duration {
	if c.ConnectionTimeout == "" {
		return 10 * time.Second
	}
	d, err := time.ParseDuration(c.ConnectionTimeout)
	if err != nil {
		return 10 * time.Second
	}
	return d
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
	AuthURL      string   `yaml:"auth-url"`
	TokenURL     string   `yaml:"token-url"`
	APIURL       string   `yaml:"api-url"`
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
	LDAPGroups    []string `yaml:"ldap-groups"`
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

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// validAuthTypes is the set of recognised authentication type strings.
var validAuthTypes = []string{"basic", "oidc", "oauth2", "ldap"}

// Validate checks that the configuration is semantically consistent.
// It returns a descriptive error for the first violation found.
func (c *Config) Validate() error {
	if !c.Auth.Enabled {
		return nil
	}

	if len(c.Auth.Types) == 0 {
		return fmt.Errorf("auth.enabled is true but auth.types is empty — specify at least one of: basic, oidc, oauth2")
	}

	for _, t := range c.Auth.Types {
		if !slices.Contains(validAuthTypes, t) {
			return fmt.Errorf("auth.types contains unrecognised value %q — valid values are: basic, oidc, oauth2", t)
		}
	}

	if slices.Contains(c.Auth.Types, "basic") {
		if len(c.Auth.Basic.Users) == 0 {
			return fmt.Errorf("auth.types includes \"basic\" but auth.basic.users is empty — add at least one user")
		}
	}

	if slices.Contains(c.Auth.Types, "oidc") {
		if c.Auth.OIDC.RedirectURL == "" {
			return fmt.Errorf("auth.types includes \"oidc\" but auth.oidc.redirect-url is empty")
		}
		if len(c.Auth.OIDC.Providers) == 0 {
			return fmt.Errorf("auth.types includes \"oidc\" but auth.oidc.providers is empty — add at least one provider")
		}
		for i, p := range c.Auth.OIDC.Providers {
			if p.Issuer == "" {
				return fmt.Errorf("auth.oidc.providers[%d] (%q): issuer must not be empty", i, p.Name)
			}
			if p.ClientID == "" {
				return fmt.Errorf("auth.oidc.providers[%d] (%q): client-id must not be empty", i, p.Name)
			}
		}
	}

	if slices.Contains(c.Auth.Types, "ldap") {
		if c.Auth.LDAP.URL == "" {
			return fmt.Errorf("auth.types includes \"ldap\" but auth.ldap.url is empty")
		}
		if c.Auth.LDAP.BindDN == "" {
			return fmt.Errorf("auth.types includes \"ldap\" but auth.ldap.bind-dn is empty")
		}
		if c.Auth.LDAP.SearchBase == "" {
			return fmt.Errorf("auth.types includes \"ldap\" but auth.ldap.search-base is empty")
		}
		if c.Auth.LDAP.ConnectionTimeout != "" {
			if _, err := time.ParseDuration(c.Auth.LDAP.ConnectionTimeout); err != nil {
				return fmt.Errorf("auth.ldap.connection-timeout %q is not a valid duration", c.Auth.LDAP.ConnectionTimeout)
			}
		}
	}

	if slices.Contains(c.Auth.Types, "oauth2") {
		if c.Auth.OAuth2.RedirectURL == "" {
			return fmt.Errorf("auth.types includes \"oauth2\" but auth.oauth2.redirect-url is empty")
		}
		if len(c.Auth.OAuth2.Providers) == 0 {
			return fmt.Errorf("auth.types includes \"oauth2\" but auth.oauth2.providers is empty — add at least one provider")
		}
		for i, p := range c.Auth.OAuth2.Providers {
			if p.ClientID == "" {
				return fmt.Errorf("auth.oauth2.providers[%d] (%q): client-id must not be empty", i, p.Name)
			}
			if p.ClientSecret == "" {
				return fmt.Errorf("auth.oauth2.providers[%d] (%q): client-secret must not be empty", i, p.Name)
			}
		}
	}

	return nil
}
