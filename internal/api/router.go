package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/Smyrcu/KafkaUI/internal/api/handlers"
	"github.com/Smyrcu/KafkaUI/internal/api/middleware"
	"github.com/Smyrcu/KafkaUI/internal/api/ws"
)

func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Recoverer)
	r.Use(chimw.RequestID)
	r.Use(middleware.Logger(deps.Logger))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		MaxAge:           300,
	}))
	r.Use(middleware.MaxBodySize(5 << 20))

	clusterHandler := handlers.NewClusterHandler(deps.Registry)
	brokerHandler := handlers.NewBrokerHandler(deps.Registry)
	topicHandler := handlers.NewTopicHandler(deps.Registry)
	messageHandler := handlers.NewMessageHandler(deps.Registry, deps.MaskingEngine)
	consumerGroupHandler := handlers.NewConsumerGroupHandler(deps.Registry)
	schemaHandler := handlers.NewSchemaHandler(deps.Registry)
	connectHandler := handlers.NewConnectHandler(deps.Registry)
	ksqlHandler := handlers.NewKsqlHandler(deps.Registry)
	aclHandler := handlers.NewACLHandler(deps.Registry)
	userHandler := handlers.NewUserHandler(deps.Registry)
	dashboardHandler := handlers.NewDashboardHandler(deps.Registry)
	metricsHandler := handlers.NewMetricsHandler(deps.MetricsStore)
	liveTailHandler := ws.NewLiveTailHandler(deps.Registry, deps.Logger)

	adminHandler := handlers.NewAdminHandler(deps.Registry, deps.DynamicCfg, deps.StaticClusterNames)
	adminUsersHandler := handlers.NewAdminUsersHandler(deps.UserStore)
	healthHandler := handlers.NewHealthHandler(deps.Registry)

	authHandler := handlers.NewAuthHandler(handlers.AuthHandlerDeps{
		Providers:    deps.Providers,
		ProviderList: deps.ProviderList,
		Basic:        deps.BasicAuth,
		RateLimiter:  deps.RateLimiter,
		Sessions:     deps.Sessions,
		UserStore:    deps.UserStore,
		RBAC:         deps.RBAC,
		AutoRules:    deps.AutoAssignment,
		DefaultRole:  deps.DefaultRole,
		Logger:       deps.Logger,
		Enabled:      deps.AuthEnabled,
		AuthTypes:    deps.AuthTypes,
	})

	// Health probes — top-level, no auth
	r.Get("/healthz", healthHandler.Liveness)
	r.Get("/readyz", healthHandler.Readiness)
	r.Get("/readyz/{service}", healthHandler.ServiceCheck)

	// Debug endpoints (dev only)
	if deps.MockMetrics != nil {
		r.Get("/debug/mock-metrics", deps.MockMetrics.ServeHTTP)
	}

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/docs", handlers.SwaggerUI)
		r.Get("/docs/openapi.yaml", handlers.SwaggerSpec)

		// Auth endpoints — no auth middleware (must be accessible unauthenticated)
		r.Get("/auth/status", authHandler.Status)
		r.Post("/auth/login", authHandler.LoginBasic)
		r.Get("/auth/login/{provider}", authHandler.LoginProvider)
		r.Get("/auth/callback", authHandler.Callback)
		r.Get("/auth/me", authHandler.Me)
		r.Post("/auth/logout", authHandler.Logout)
		r.Get("/auth/permissions", authHandler.Permissions)

		// Protected API routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(deps.Sessions, deps.AuthEnabled))

			// Helper closure for RBAC middleware
			rbacDeps := middleware.RBACDeps{
				RBAC: deps.RBAC, Store: deps.UserStore,
				AutoRules: deps.AutoAssignment, DefaultRole: deps.DefaultRole,
			}
			requireAction := func(action string) func(http.Handler) http.Handler {
				return middleware.RequireAction(rbacDeps, action, deps.AuthEnabled)
			}

			r.With(requireAction("view_dashboard")).Get("/dashboard", dashboardHandler.Overview)
			r.With(requireAction("view_dashboard")).Get("/clusters", clusterHandler.List)

			r.Route("/admin", func(r chi.Router) {
				r.Use(middleware.RequireRole("admin", deps.AuthEnabled, deps.UserStore))
				r.Get("/clusters", adminHandler.ListClusters)
				r.Post("/clusters", adminHandler.AddCluster)
				r.Post("/clusters/test", adminHandler.TestConnection)
				r.Put("/clusters/{name}", adminHandler.UpdateCluster)
				r.Delete("/clusters/{name}", adminHandler.DeleteCluster)
				r.Get("/users", adminUsersHandler.List)
				r.Get("/users/{id}", adminUsersHandler.Get)
				r.Put("/users/{id}/roles", adminUsersHandler.SetRoles)
				r.Delete("/users/{id}", adminUsersHandler.Delete)
			})

			r.Route("/clusters/{clusterName}", func(r chi.Router) {
				r.With(requireAction("view_dashboard")).Get("/overview", dashboardHandler.ClusterOverviewDetail)
				r.With(requireAction("view_brokers")).Get("/brokers", brokerHandler.List)

				r.With(requireAction("view_topics")).Get("/topics", topicHandler.List)
				r.With(requireAction("create_topics")).Post("/topics", topicHandler.Create)
				r.With(requireAction("view_topics")).Get("/topics/{topicName}", topicHandler.Details)
				r.With(requireAction("delete_topics")).Delete("/topics/{topicName}", topicHandler.Delete)

				r.With(requireAction("view_messages")).Get("/topics/{topicName}/messages", messageHandler.Browse)
				r.With(requireAction("produce_messages")).Post("/topics/{topicName}/messages", messageHandler.Produce)

				r.With(requireAction("view_consumer_groups")).Get("/consumer-groups", consumerGroupHandler.List)
				r.With(requireAction("view_consumer_groups")).Get("/consumer-groups/{groupName}", consumerGroupHandler.Details)
				r.With(requireAction("reset_consumer_groups")).Post("/consumer-groups/{groupName}/reset", consumerGroupHandler.ResetOffsets)

				r.With(requireAction("view_schemas")).Get("/schemas", schemaHandler.List)
				r.With(requireAction("create_schemas")).Post("/schemas", schemaHandler.Create)
				r.With(requireAction("view_schemas")).Get("/schemas/{subject}", schemaHandler.Details)
				r.With(requireAction("delete_schemas")).Delete("/schemas/{subject}", schemaHandler.Delete)

				r.With(requireAction("view_connectors")).Get("/connectors", connectHandler.List)
				r.With(requireAction("manage_connectors")).Post("/connectors", connectHandler.Create)
				r.With(requireAction("view_connectors")).Get("/connectors/{connectorName}", connectHandler.Details)
				r.With(requireAction("manage_connectors")).Put("/connectors/{connectorName}", connectHandler.Update)
				r.With(requireAction("manage_connectors")).Delete("/connectors/{connectorName}", connectHandler.Delete)
				r.With(requireAction("manage_connectors")).Post("/connectors/{connectorName}/restart", connectHandler.Restart)
				r.With(requireAction("manage_connectors")).Post("/connectors/{connectorName}/pause", connectHandler.Pause)
				r.With(requireAction("manage_connectors")).Post("/connectors/{connectorName}/resume", connectHandler.Resume)

				r.With(requireAction("execute_ksql")).Post("/ksql", ksqlHandler.Execute)
				r.With(requireAction("view_ksql")).Get("/ksql/info", ksqlHandler.Info)

				r.With(requireAction("view_acls")).Get("/acls", aclHandler.List)
				r.With(requireAction("create_acls")).Post("/acls", aclHandler.Create)
				r.With(requireAction("delete_acls")).Post("/acls/delete", aclHandler.Delete)

				r.With(requireAction("view_kafka_users")).Get("/users", userHandler.List)
				r.With(requireAction("manage_kafka_users")).Post("/users", userHandler.Create)
				r.With(requireAction("manage_kafka_users")).Post("/users/delete", userHandler.Delete)

				r.With(requireAction("view_dashboard")).Get("/metrics", metricsHandler.Metrics)
			})
		})
	})

	r.Route("/ws", func(r chi.Router) {
		r.Use(middleware.Auth(deps.Sessions, deps.AuthEnabled))
		wsRBACDeps := middleware.RBACDeps{
			RBAC: deps.RBAC, Store: deps.UserStore,
			AutoRules: deps.AutoAssignment, DefaultRole: deps.DefaultRole,
		}
		r.With(middleware.RequireAction(wsRBACDeps, "view_messages", deps.AuthEnabled)).
			Get("/clusters/{clusterName}/topics/{topicName}/live", liveTailHandler.Handle)
	})

	return r
}
