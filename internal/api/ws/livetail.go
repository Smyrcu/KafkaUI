package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/twmb/franz-go/pkg/kgo"

	celfilter "github.com/Smyrcu/KafkaUI/internal/cel"
	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type wsMessage struct {
	Partition int32             `json:"partition"`
	Offset    int64             `json:"offset"`
	Timestamp time.Time         `json:"timestamp"`
	Key       string            `json:"key"`
	Value     string            `json:"value"`
	Headers   map[string]string `json:"headers,omitempty"`
}

type controlMessage struct {
	Action string `json:"action"`
}

type LiveTailHandler struct {
	registry *kafka.Registry
	logger   *slog.Logger
}

func NewLiveTailHandler(registry *kafka.Registry, logger *slog.Logger) *LiveTailHandler {
	return &LiveTailHandler{registry: registry, logger: logger}
}

func (h *LiveTailHandler) Handle(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "clusterName")
	topicName := chi.URLParam(r, "topicName")

	client, ok := h.registry.Get(clusterName)
	if !ok {
		http.Error(w, "cluster not found", http.StatusNotFound)
		return
	}

	// Parse and compile CEL filter before upgrading to WebSocket
	var filter *celfilter.Filter
	if filterExpr := r.URL.Query().Get("filter"); filterExpr != "" {
		var filterErr error
		filter, filterErr = celfilter.NewFilter(filterExpr)
		if filterErr != nil {
			http.Error(w, filterErr.Error(), http.StatusBadRequest)
			return
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	cfg := client.Config()
	seeds := strings.Split(cfg.BootstrapServers, ",")
	opts := []kgo.Opt{
		kgo.SeedBrokers(seeds...),
		kgo.ConsumeTopics(topicName),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
	}
	if cfg.SASL.Mechanism != "" {
		saslOpt, saslErr := kafka.BuildSASLOpt(cfg.SASL)
		if saslErr != nil {
			h.logger.Error("SASL config failed for live tail", "error", saslErr)
			return
		}
		opts = append(opts, saslOpt)
	}
	if cfg.TLS.Enabled {
		tlsOpt, tlsErr := kafka.BuildTLSOpt(cfg.TLS)
		if tlsErr != nil {
			h.logger.Error("TLS config failed for live tail", "error", tlsErr)
			return
		}
		opts = append(opts, tlsOpt)
	}

	consumer, err := kgo.NewClient(opts...)
	if err != nil {
		h.logger.Error("failed to create live tail consumer", "error", err)
		return
	}
	defer consumer.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Read control messages from client
	go func() {
		defer cancel()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var ctrl controlMessage
			if json.Unmarshal(msg, &ctrl) == nil && ctrl.Action == "stop" {
				return
			}
		}
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		default:
			fetchCtx, fetchCancel := context.WithTimeout(ctx, 2*time.Second)
			fetches := consumer.PollFetches(fetchCtx)
			fetchCancel()

			if ctx.Err() != nil {
				return
			}

			fetches.EachRecord(func(r *kgo.Record) {
				headers := make(map[string]string)
				for _, hdr := range r.Headers {
					headers[hdr.Key] = string(hdr.Value)
				}
				msg := wsMessage{
					Partition: r.Partition,
					Offset:    r.Offset,
					Timestamp: r.Timestamp,
					Key:       string(r.Key),
					Value:     string(r.Value),
					Headers:   headers,
				}

				// Apply CEL filter if provided
				if filter != nil {
					record := kafka.MessageRecord{
						Partition: msg.Partition,
						Offset:    msg.Offset,
						Timestamp: msg.Timestamp,
						Key:       msg.Key,
						Value:     msg.Value,
						Headers:   msg.Headers,
					}
					matched, matchErr := filter.Match(record)
					if matchErr != nil {
						h.logger.Error("CEL filter evaluation failed", "error", matchErr)
						return
					}
					if !matched {
						return
					}
				}

				data, _ := json.Marshal(msg)
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					cancel()
				}
			})
		}
	}
}
