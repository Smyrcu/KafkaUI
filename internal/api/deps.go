package api

import (
	"log/slog"
	"net/http"

	"github.com/Smyrcu/KafkaUI/internal/auth"
	"github.com/Smyrcu/KafkaUI/internal/config"
	"github.com/Smyrcu/KafkaUI/internal/kafka"
	"github.com/Smyrcu/KafkaUI/internal/masking"
	"github.com/Smyrcu/KafkaUI/internal/metrics"
)

// RouterDeps bundles every dependency needed by NewRouter,
// keeping the function signature stable as new deps are added.
type RouterDeps struct {
	Registry           *kafka.Registry
	Logger             *slog.Logger
	Sessions           *auth.SessionManager
	AuthEnabled        bool
	MaskingEngine      *masking.Engine
	Providers          map[string]auth.IdentityProvider
	ProviderList       []auth.ProviderInfo
	BasicAuth          *auth.BasicAuthenticator
	RateLimiter        *auth.LoginRateLimiter
	AuthTypes          []string
	MetricsStore       *metrics.Store
	MockMetrics        http.Handler
	DynamicCfg         *config.DynamicConfig
	StaticClusterNames []string
	UserStore          *auth.UserStore
	RBAC               *auth.RBAC
	AutoAssignment     []config.AutoAssignmentRule
	DefaultRole        string
	TrustProxy         bool
	CORSOrigins        []string
	SerDeChains        map[string]kafka.SerDeChain
}
