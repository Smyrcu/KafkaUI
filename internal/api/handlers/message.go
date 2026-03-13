package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	celfilter "github.com/Smyrcu/KafkaUI/internal/cel"
	"github.com/Smyrcu/KafkaUI/internal/kafka"
	"github.com/Smyrcu/KafkaUI/internal/masking"
)

type MessageHandler struct {
	registry      *kafka.Registry
	maskingEngine *masking.Engine
}

func NewMessageHandler(reg *kafka.Registry, maskingEngine *masking.Engine) *MessageHandler {
	return &MessageHandler{registry: reg, maskingEngine: maskingEngine}
}

func (h *MessageHandler) Browse(w http.ResponseWriter, r *http.Request) {
	topicName := chi.URLParam(r, "topicName")

	client, ok := getClient(h.registry, w, r)
	if !ok {
		return
	}

	req := kafka.ConsumeRequest{
		Offset: -2, // earliest by default for browsing
		Limit:  100,
	}

	if v := r.URL.Query().Get("partition"); v != "" {
		p, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid partition parameter")
			return
		}
		p32 := int32(p)
		req.Partition = &p32
	}

	if v := r.URL.Query().Get("offset"); v != "" {
		switch v {
		case "earliest":
			req.Offset = -2
		case "latest":
			req.Offset = -1
		default:
			o, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid offset parameter")
				return
			}
			req.Offset = o
		}
	}

	if v := r.URL.Query().Get("limit"); v != "" {
		l, err := strconv.Atoi(v)
		if err != nil || l < 1 || l > 500 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 500")
			return
		}
		req.Limit = l
	}

	if v := r.URL.Query().Get("timestamp"); v != "" {
		ts, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid timestamp (use RFC3339 format)")
			return
		}
		req.Timestamp = &ts
	}

	// Compile CEL filter if provided.
	var filter *celfilter.Filter
	filterExpr := r.URL.Query().Get("filter")
	if filterExpr != "" {
		var err error
		filter, err = celfilter.NewFilter(filterExpr)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	// When filtering, over-fetch from Kafka since some messages will be
	// discarded. Save the original requested limit for the final result.
	originalLimit := req.Limit
	if filter != nil {
		req.Limit = originalLimit * 5
		if req.Limit > 2500 {
			req.Limit = 2500
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	messages, err := client.ConsumeMessages(ctx, topicName, req)
	if err != nil {
		writeInternalError(w, "consuming messages", err)
		return
	}

	if h.maskingEngine != nil {
		for i := range messages {
			messages[i].Value = h.maskingEngine.MaskMessage(topicName, messages[i].Value)
		}
	}

	// Apply CEL filter after masking.
	if filter != nil {
		filtered := make([]kafka.MessageRecord, 0, originalLimit)
		for _, msg := range messages {
			match, err := filter.Match(msg)
			if err != nil {
				continue
			}
			if match {
				filtered = append(filtered, msg)
				if len(filtered) >= originalLimit {
					break
				}
			}
		}
		messages = filtered
	}

	writeJSON(w, http.StatusOK, messages)
}

func (h *MessageHandler) Produce(w http.ResponseWriter, r *http.Request) {
	topicName := chi.URLParam(r, "topicName")

	client, ok := getClient(h.registry, w, r)
	if !ok {
		return
	}

	var req kafka.ProduceRequest
	if !decodeBody(w, r, &req) {
		return
	}

	record, err := client.ProduceMessage(r.Context(), topicName, req)
	if err != nil {
		writeInternalError(w, "producing message", err)
		return
	}

	writeJSON(w, http.StatusOK, record)
}
