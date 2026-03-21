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
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		host := r.Host
		if host == "" {
			host = r.Header.Get("Host")
		}
		// Allow same-origin requests: origin must end with the host
		return strings.HasSuffix(origin, "://"+host)
	},
}

type controlMessage struct {
	Action string `json:"action"`
}

type LiveTailHandler struct {
	registry    *kafka.Registry
	logger      *slog.Logger
	serdeChains map[string]kafka.SerDeChain
}

func NewLiveTailHandler(registry *kafka.Registry, logger *slog.Logger, serdeChains map[string]kafka.SerDeChain) *LiveTailHandler {
	return &LiveTailHandler{registry: registry, logger: logger, serdeChains: serdeChains}
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

	opts, err := kafka.BuildBaseOpts(client.Config())
	if err != nil {
		h.logger.Error("failed to build consumer opts for live tail", "error", err)
		return
	}
	opts = append(opts,
		kgo.ConsumeTopics(topicName),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
	)

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
				key := string(r.Key)
				value := string(r.Value)
				var keyFmt, valFmt string
				if chain, ok := h.serdeChains[clusterName]; ok && chain != nil {
					key, keyFmt = chain.DeserializeWithFormat(topicName, r.Key, headers)
					value, valFmt = chain.DeserializeWithFormat(topicName, r.Value, headers)
				}
				msg := kafka.MessageRecord{
					Partition:   r.Partition,
					Offset:      r.Offset,
					Timestamp:   r.Timestamp,
					Key:         key,
					Value:       value,
					Headers:     headers,
					KeyFormat:   keyFmt,
					ValueFormat: valFmt,
				}

				// Apply CEL filter if provided
				if filter != nil {
					matched, matchErr := filter.Match(msg)
					if matchErr != nil {
						h.logger.Error("CEL filter evaluation failed", "error", matchErr)
						return
					}
					if !matched {
						return
					}
				}

				data, err := json.Marshal(msg)
				if err != nil {
					h.logger.Error("failed to marshal live tail message", "error", err)
					return
				}
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					cancel()
				}
			})
		}
	}
}
