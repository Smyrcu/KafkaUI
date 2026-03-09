package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a reusable HTTP client for JSON-based APIs. It handles request
// encoding, response decoding, and error formatting with configurable
// content types and error prefixes.
type Client struct {
	baseURL     string
	httpClient  *http.Client
	contentType string
	accept      string
	errorPrefix string
}

// New creates a Client with the given settings. The baseURL should not have a
// trailing slash (callers are expected to trim it before passing).
func New(baseURL string, timeout time.Duration, contentType, accept, errorPrefix string) *Client {
	return &Client{
		baseURL:     baseURL,
		httpClient:  &http.Client{Timeout: timeout},
		contentType: contentType,
		accept:      accept,
		errorPrefix: errorPrefix,
	}
}

// Do performs an HTTP request. If body is non-nil it is JSON-encoded and sent
// as the request body. If dest is non-nil the response body is JSON-decoded
// into it.
func (c *Client) Do(ctx context.Context, method, path string, body any, dest any) error {
	var bodyReader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if c.accept != "" {
		req.Header.Set("Accept", c.accept)
	}
	if body != nil {
		req.Header.Set("Content-Type", c.contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", c.errorPrefix, err)
	}
	defer resp.Body.Close()

	const maxResponseBody = 16 << 20 // 16 MB
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s (%d): %s", c.errorPrefix, resp.StatusCode, string(respBody))
	}

	if dest != nil {
		if err := json.Unmarshal(respBody, dest); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// DoRaw performs an HTTP request and returns the raw response body. This is
// useful when the caller needs to handle non-standard JSON structures or
// needs the raw bytes for custom unmarshalling.
func (c *Client) DoRaw(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.accept != "" {
		req.Header.Set("Accept", c.accept)
	}
	if body != nil {
		req.Header.Set("Content-Type", c.contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", c.errorPrefix, err)
	}
	defer resp.Body.Close()

	const maxResponseBody = 16 << 20 // 16 MB
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s (%d): %s", c.errorPrefix, resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
