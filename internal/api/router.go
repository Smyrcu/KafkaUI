package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/Smyrcu/KafkaUI/internal/api/handlers"
	"github.com/Smyrcu/KafkaUI/internal/api/middleware"
	"github.com/Smyrcu/KafkaUI/internal/api/ws"
	"github.com/Smyrcu/KafkaUI/internal/auth"
	"github.com/Smyrcu/KafkaUI/internal/kafka"
	"github.com/Smyrcu/KafkaUI/internal/masking"
)

func NewRouter(registry *kafka.Registry, logger *slog.Logger, sessions *auth.SessionManager, authEnabled bool, maskingEngine *masking.Engine, authProvider *auth.Provider, basicAuth *auth.BasicAuthenticator, rateLimiter *auth.LoginRateLimiter, authType string) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Recoverer)
	r.Use(chimw.RequestID)
	r.Use(middleware.Logger(logger))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		MaxAge:           300,
	}))

	clusterHandler := handlers.NewClusterHandler(registry)
	brokerHandler := handlers.NewBrokerHandler(registry)
	topicHandler := handlers.NewTopicHandler(registry)
	messageHandler := handlers.NewMessageHandler(registry, maskingEngine)
	consumerGroupHandler := handlers.NewConsumerGroupHandler(registry)
	schemaHandler := handlers.NewSchemaHandler(registry)
	connectHandler := handlers.NewConnectHandler(registry)
	ksqlHandler := handlers.NewKsqlHandler(registry)
	aclHandler := handlers.NewACLHandler(registry)
	userHandler := handlers.NewUserHandler(registry)
	dashboardHandler := handlers.NewDashboardHandler(registry)
	liveTailHandler := ws.NewLiveTailHandler(registry, logger)

	authHandler := handlers.NewAuthHandler(authProvider, basicAuth, rateLimiter, sessions, logger, authEnabled, authType)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/docs", handlers.SwaggerUI)
		r.Get("/docs/openapi.yaml", handlers.SwaggerSpec)

		// Auth endpoints — no auth middleware (must be accessible unauthenticated)
		r.Get("/auth/status", authHandler.Status)
		r.Post("/auth/login", authHandler.LoginBasic)
		r.Get("/auth/login", authHandler.LoginOIDC)
		r.Get("/auth/callback", authHandler.Callback)
		r.Get("/auth/me", authHandler.Me)
		r.Post("/auth/logout", authHandler.Logout)

		// Protected API routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(sessions, authEnabled))

			r.Get("/dashboard", dashboardHandler.Overview)
			r.Get("/clusters", clusterHandler.List)

			r.Route("/clusters/{clusterName}", func(r chi.Router) {
				r.Get("/overview", dashboardHandler.ClusterOverviewDetail)
				r.Get("/brokers", brokerHandler.List)

				r.Get("/topics", topicHandler.List)
				r.Post("/topics", topicHandler.Create)
				r.Get("/topics/{topicName}", topicHandler.Details)
				r.Delete("/topics/{topicName}", topicHandler.Delete)

				r.Get("/topics/{topicName}/messages", messageHandler.Browse)
				r.Post("/topics/{topicName}/messages", messageHandler.Produce)

				r.Get("/consumer-groups", consumerGroupHandler.List)
				r.Get("/consumer-groups/{groupName}", consumerGroupHandler.Details)
				r.Post("/consumer-groups/{groupName}/reset", consumerGroupHandler.ResetOffsets)

				r.Get("/schemas", schemaHandler.List)
				r.Post("/schemas", schemaHandler.Create)
				r.Get("/schemas/{subject}", schemaHandler.Details)
				r.Delete("/schemas/{subject}", schemaHandler.Delete)

				r.Get("/connectors", connectHandler.List)
				r.Post("/connectors", connectHandler.Create)
				r.Get("/connectors/{connectorName}", connectHandler.Details)
				r.Put("/connectors/{connectorName}", connectHandler.Update)
				r.Delete("/connectors/{connectorName}", connectHandler.Delete)
				r.Post("/connectors/{connectorName}/restart", connectHandler.Restart)
				r.Post("/connectors/{connectorName}/pause", connectHandler.Pause)
				r.Post("/connectors/{connectorName}/resume", connectHandler.Resume)

				r.Post("/ksql", ksqlHandler.Execute)
				r.Get("/ksql/info", ksqlHandler.Info)

				r.Get("/acls", aclHandler.List)
				r.Post("/acls", aclHandler.Create)
				r.Post("/acls/delete", aclHandler.Delete)

				r.Get("/users", userHandler.List)
				r.Post("/users", userHandler.Create)
				r.Post("/users/delete", userHandler.Delete)
			})
		})
	})

	r.Route("/ws", func(r chi.Router) {
		r.Use(middleware.Auth(sessions, authEnabled))
		r.Get("/clusters/{clusterName}/topics/{topicName}/live", liveTailHandler.Handle)
	})

	return r
}
