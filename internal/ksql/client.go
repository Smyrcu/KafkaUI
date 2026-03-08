package ksql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Smyrcu/KafkaUI/internal/httpclient"
)

type Client struct {
	http *httpclient.Client
}

type ExecuteRequest struct {
	Query string `json:"query"`
}

type ExecuteResponse struct {
	Type          string          `json:"type"`
	StatementText string          `json:"statementText,omitempty"`
	Warnings      []Warning       `json:"warnings,omitempty"`
	Data          json.RawMessage `json:"data"`
}

type Warning struct {
	Message string `json:"message"`
}

func NewClient(baseURL string) *Client {
	return &Client{
		http: httpclient.New(
			strings.TrimRight(baseURL, "/"),
			30*time.Second,
			"application/vnd.ksql.v1+json",
			"application/vnd.ksql.v1+json",
			"ksql error",
		),
	}
}

// Execute sends a KSQL statement to the server and returns the parsed response.
func (c *Client) Execute(ctx context.Context, query string) (*ExecuteResponse, error) {
	reqBody := map[string]any{
		"ksql":              query,
		"streamsProperties": map[string]any{},
	}

	encoded, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	respBody, err := c.http.DoRaw(ctx, "POST", "/ksql", bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("execute ksql: %w", err)
	}

	var elements []json.RawMessage
	if err := json.Unmarshal(respBody, &elements); err != nil {
		return nil, fmt.Errorf("decode response array: %w", err)
	}

	if len(elements) == 0 {
		return &ExecuteResponse{}, nil
	}

	first := elements[0]

	// Extract fields from the first element.
	var parsed struct {
		Type          string    `json:"@type"`
		StatementText string    `json:"statementText"`
		Warnings      []Warning `json:"warnings"`
	}
	if err := json.Unmarshal(first, &parsed); err != nil {
		return nil, fmt.Errorf("decode first element: %w", err)
	}

	return &ExecuteResponse{
		Type:          parsed.Type,
		StatementText: parsed.StatementText,
		Warnings:      parsed.Warnings,
		Data:          first,
	}, nil
}

// Info returns KSQL server info (version, cluster ID, etc.) as a generic map.
func (c *Client) Info(ctx context.Context) (map[string]any, error) {
	respBody, err := c.http.DoRaw(ctx, "GET", "/info", nil)
	if err != nil {
		return nil, fmt.Errorf("get ksql info: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode info response: %w", err)
	}

	return result, nil
}
