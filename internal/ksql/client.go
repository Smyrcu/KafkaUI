package ksql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
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
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
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

	respBody, err := c.doRequest(ctx, http.MethodPost, "/ksql", bytes.NewReader(encoded))
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
	respBody, err := c.doRequest(ctx, http.MethodGet, "/info", nil)
	if err != nil {
		return nil, fmt.Errorf("get ksql info: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode info response: %w", err)
	}

	return result, nil
}

// doRequest is a shared HTTP helper that creates a request, sets KSQL-specific
// headers, executes it, checks the status code, and returns the raw response body.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.ksql.v1+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/vnd.ksql.v1+json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ksql error (%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
