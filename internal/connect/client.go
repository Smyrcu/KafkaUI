package connect

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	name       string
	httpClient *http.Client
}

type ConnectorInfo struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	State          string `json:"state"`
	WorkerID       string `json:"workerId"`
	ConnectCluster string `json:"connectCluster"`
}

type ConnectorDetail struct {
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	State          string            `json:"state"`
	WorkerID       string            `json:"workerId"`
	Config         map[string]string `json:"config"`
	Tasks          []TaskStatus      `json:"tasks"`
	ConnectCluster string            `json:"connectCluster"`
}

type TaskStatus struct {
	ID       int    `json:"id"`
	State    string `json:"state"`
	WorkerID string `json:"workerId"`
	Trace    string `json:"trace,omitempty"`
}

type CreateConnectorRequest struct {
	Name   string            `json:"name"`
	Config map[string]string `json:"config"`
}

func NewClient(name, baseURL string) *Client {
	return &Client{
		name:       name,
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// ListConnectors returns all connectors with their info and status.
func (c *Client) ListConnectors(ctx context.Context) ([]ConnectorInfo, error) {
	var expanded map[string]struct {
		Info struct {
			Name   string            `json:"name"`
			Config map[string]string `json:"config"`
			Type   string            `json:"type"`
		} `json:"info"`
		Status struct {
			Name      string `json:"name"`
			Connector struct {
				State    string `json:"state"`
				WorkerID string `json:"worker_id"`
			} `json:"connector"`
		} `json:"status"`
	}

	if err := c.doJSON(ctx, http.MethodGet, "/connectors?expand=info&expand=status", nil, &expanded); err != nil {
		return nil, fmt.Errorf("list connectors: %w", err)
	}

	results := make([]ConnectorInfo, 0, len(expanded))
	for name, entry := range expanded {
		results = append(results, ConnectorInfo{
			Name:           name,
			Type:           entry.Info.Type,
			State:          entry.Status.Connector.State,
			WorkerID:       entry.Status.Connector.WorkerID,
			ConnectCluster: c.name,
		})
	}

	return results, nil
}

// GetConnector returns full detail for a connector including config and task status.
func (c *Client) GetConnector(ctx context.Context, name string) (*ConnectorDetail, error) {
	escaped := url.PathEscape(name)

	// Get connector config and type.
	var connResp struct {
		Name   string            `json:"name"`
		Config map[string]string `json:"config"`
		Type   string            `json:"type"`
	}
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/connectors/%s", escaped), nil, &connResp); err != nil {
		return nil, fmt.Errorf("get connector %q: %w", name, err)
	}

	// Get connector status.
	var statusResp struct {
		Name      string `json:"name"`
		Connector struct {
			State    string `json:"state"`
			WorkerID string `json:"worker_id"`
		} `json:"connector"`
		Tasks []struct {
			ID       int    `json:"id"`
			State    string `json:"state"`
			WorkerID string `json:"worker_id"`
			Trace    string `json:"trace"`
		} `json:"tasks"`
	}
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/connectors/%s/status", escaped), nil, &statusResp); err != nil {
		return nil, fmt.Errorf("get connector status %q: %w", name, err)
	}

	tasks := make([]TaskStatus, 0, len(statusResp.Tasks))
	for _, t := range statusResp.Tasks {
		tasks = append(tasks, TaskStatus{
			ID:       t.ID,
			State:    t.State,
			WorkerID: t.WorkerID,
			Trace:    t.Trace,
		})
	}

	return &ConnectorDetail{
		Name:           connResp.Name,
		Type:           connResp.Type,
		State:          statusResp.Connector.State,
		WorkerID:       statusResp.Connector.WorkerID,
		Config:         connResp.Config,
		Tasks:          tasks,
		ConnectCluster: c.name,
	}, nil
}

// CreateConnector creates a new connector and returns its full detail.
func (c *Client) CreateConnector(ctx context.Context, req CreateConnectorRequest) (*ConnectorDetail, error) {
	body := map[string]any{
		"name":   req.Name,
		"config": req.Config,
	}

	var createResp struct {
		Name   string            `json:"name"`
		Config map[string]string `json:"config"`
		Type   string            `json:"type"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/connectors", body, &createResp); err != nil {
		return nil, fmt.Errorf("create connector %q: %w", req.Name, err)
	}

	return c.GetConnector(ctx, createResp.Name)
}

// UpdateConnector updates the config for an existing connector and returns its full detail.
func (c *Client) UpdateConnector(ctx context.Context, name string, config map[string]string) (*ConnectorDetail, error) {
	escaped := url.PathEscape(name)

	var updateResp map[string]string
	if err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/connectors/%s/config", escaped), config, &updateResp); err != nil {
		return nil, fmt.Errorf("update connector %q: %w", name, err)
	}

	return c.GetConnector(ctx, name)
}

// DeleteConnector deletes a connector.
func (c *Client) DeleteConnector(ctx context.Context, name string) error {
	escaped := url.PathEscape(name)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+fmt.Sprintf("/connectors/%s", escaped), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete connector %q: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kafka connect error (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// RestartConnector restarts a connector.
func (c *Client) RestartConnector(ctx context.Context, name string) error {
	return c.doAction(ctx, name, "restart")
}

// PauseConnector pauses a connector.
func (c *Client) PauseConnector(ctx context.Context, name string) error {
	return c.doAction(ctx, name, "pause")
}

// ResumeConnector resumes a paused connector.
func (c *Client) ResumeConnector(ctx context.Context, name string) error {
	return c.doAction(ctx, name, "resume")
}

// doAction performs a POST action (restart, pause, resume) on a connector.
func (c *Client) doAction(ctx context.Context, name, action string) error {
	escaped := url.PathEscape(name)
	path := fmt.Sprintf("/connectors/%s/%s", escaped, action)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s connector %q: %w", action, name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kafka connect error (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// doJSON is a helper that performs an HTTP request and decodes the JSON response.
// If body is non-nil, it is JSON-encoded and sent as the request body.
// If dest is non-nil, the response body is JSON-decoded into it.
func (c *Client) doJSON(ctx context.Context, method, path string, body any, dest any) error {
	var bodyReader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = strings.NewReader(string(encoded))
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("kafka connect error (%d): %s", resp.StatusCode, string(respBody))
	}

	if dest != nil {
		if err := json.Unmarshal(respBody, dest); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
