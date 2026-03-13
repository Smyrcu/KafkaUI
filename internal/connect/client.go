package connect

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Smyrcu/KafkaUI/internal/httpclient"
)

type Client struct {
	name string
	http *httpclient.Client
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
		name: name,
		http: httpclient.New(
			strings.TrimRight(baseURL, "/"),
			10*time.Second,
			"application/json",
			"application/json",
			"kafka connect error",
		),
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

	if err := c.http.Do(ctx, "GET", "/connectors?expand=info&expand=status", nil, &expanded); err != nil {
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
	if err := c.http.Do(ctx, "GET", fmt.Sprintf("/connectors/%s", escaped), nil, &connResp); err != nil {
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
	if err := c.http.Do(ctx, "GET", fmt.Sprintf("/connectors/%s/status", escaped), nil, &statusResp); err != nil {
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
	if err := c.http.Do(ctx, "POST", "/connectors", body, &createResp); err != nil {
		return nil, fmt.Errorf("create connector %q: %w", req.Name, err)
	}

	return c.GetConnector(ctx, createResp.Name)
}

// UpdateConnector updates the config for an existing connector and returns its full detail.
func (c *Client) UpdateConnector(ctx context.Context, name string, config map[string]string) (*ConnectorDetail, error) {
	escaped := url.PathEscape(name)

	var updateResp map[string]string
	if err := c.http.Do(ctx, "PUT", fmt.Sprintf("/connectors/%s/config", escaped), config, &updateResp); err != nil {
		return nil, fmt.Errorf("update connector %q: %w", name, err)
	}

	return c.GetConnector(ctx, name)
}

// DeleteConnector deletes a connector.
func (c *Client) DeleteConnector(ctx context.Context, name string) error {
	escaped := url.PathEscape(name)

	if _, err := c.http.DoRaw(ctx, "DELETE", fmt.Sprintf("/connectors/%s", escaped), nil); err != nil {
		return fmt.Errorf("delete connector %q: %w", name, err)
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

	if _, err := c.http.DoRaw(ctx, "POST", path, nil); err != nil {
		return fmt.Errorf("%s connector %q: %w", action, name, err)
	}

	return nil
}
