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
	OIDCProviders      map[string]*auth.OIDCProvider
	OIDCProviderCfg    []config.OIDCProvider
	BasicAuth          *auth.BasicAuthenticator
	RateLimiter        *auth.LoginRateLimiter
	AuthTypes          []string
	MetricsStore       *metrics.Store
	MockMetrics        http.Handler
	DynamicCfg         *config.DynamicConfig
	StaticClusterNames []string
}
