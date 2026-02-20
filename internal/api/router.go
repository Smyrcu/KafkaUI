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
	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

func NewRouter(registry *kafka.Registry, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Recoverer)
	r.Use(chimw.RequestID)
	r.Use(middleware.Logger(logger))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	clusterHandler := handlers.NewClusterHandler(registry)
	brokerHandler := handlers.NewBrokerHandler(registry)
	topicHandler := handlers.NewTopicHandler(registry)
	messageHandler := handlers.NewMessageHandler(registry)
	liveTailHandler := ws.NewLiveTailHandler(registry, logger)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/docs", handlers.SwaggerUI)
		r.Get("/docs/openapi.yaml", handlers.SwaggerSpec)

		r.Get("/clusters", clusterHandler.List)

		r.Route("/clusters/{clusterName}", func(r chi.Router) {
			r.Get("/brokers", brokerHandler.List)

			r.Get("/topics", topicHandler.List)
			r.Post("/topics", topicHandler.Create)
			r.Get("/topics/{topicName}", topicHandler.Details)
			r.Delete("/topics/{topicName}", topicHandler.Delete)

			r.Get("/topics/{topicName}/messages", messageHandler.Browse)
			r.Post("/topics/{topicName}/messages", messageHandler.Produce)
		})
	})

	r.Route("/ws", func(r chi.Router) {
		r.Get("/clusters/{clusterName}/topics/{topicName}/live", liveTailHandler.Handle)
	})

	return r
}
