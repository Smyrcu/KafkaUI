package schema

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
	httpClient *http.Client
}

type SubjectInfo struct {
	Subject        string `json:"subject"`
	LatestVersion  int    `json:"latestVersion"`
	LatestSchemaID int    `json:"latestSchemaId"`
	SchemaType     string `json:"schemaType"`
}

type SchemaDetail struct {
	Subject       string          `json:"subject"`
	Compatibility string          `json:"compatibility"`
	Versions      []SchemaVersion `json:"versions"`
}

type SchemaVersion struct {
	Version    int    `json:"version"`
	ID         int    `json:"id"`
	Schema     string `json:"schema"`
	SchemaType string `json:"schemaType"`
}

type CreateSchemaRequest struct {
	Subject    string `json:"subject"`
	Schema     string `json:"schema"`
	SchemaType string `json:"schemaType"`
}

type CreateSchemaResponse struct {
	ID int `json:"id"`
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// ListSubjects returns all subjects with their latest version info.
func (c *Client) ListSubjects(ctx context.Context) ([]SubjectInfo, error) {
	var subjects []string
	if err := c.doJSON(ctx, http.MethodGet, "/subjects", nil, &subjects); err != nil {
		return nil, fmt.Errorf("list subjects: %w", err)
	}

	results := make([]SubjectInfo, 0, len(subjects))
	for _, subject := range subjects {
		path := fmt.Sprintf("/subjects/%s/versions/latest", url.PathEscape(subject))

		var latest struct {
			Subject    string `json:"subject"`
			Version    int    `json:"version"`
			ID         int    `json:"id"`
			SchemaType string `json:"schemaType"`
		}
		if err := c.doJSON(ctx, http.MethodGet, path, nil, &latest); err != nil {
			return nil, fmt.Errorf("get latest version for %q: %w", subject, err)
		}

		schemaType := latest.SchemaType
		if schemaType == "" {
			schemaType = "AVRO"
		}

		results = append(results, SubjectInfo{
			Subject:        subject,
			LatestVersion:  latest.Version,
			LatestSchemaID: latest.ID,
			SchemaType:     schemaType,
		})
	}

	return results, nil
}

// GetSubjectDetails returns full version history and compatibility config for a subject.
func (c *Client) GetSubjectDetails(ctx context.Context, subject string) (*SchemaDetail, error) {
	escaped := url.PathEscape(subject)

	// Get list of version numbers.
	var versionNums []int
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/subjects/%s/versions", escaped), nil, &versionNums); err != nil {
		return nil, fmt.Errorf("list versions for %q: %w", subject, err)
	}

	// Fetch each version.
	versions := make([]SchemaVersion, 0, len(versionNums))
	for _, v := range versionNums {
		path := fmt.Sprintf("/subjects/%s/versions/%d", escaped, v)

		var ver struct {
			Version    int    `json:"version"`
			ID         int    `json:"id"`
			Schema     string `json:"schema"`
			SchemaType string `json:"schemaType"`
		}
		if err := c.doJSON(ctx, http.MethodGet, path, nil, &ver); err != nil {
			return nil, fmt.Errorf("get version %d for %q: %w", v, subject, err)
		}

		schemaType := ver.SchemaType
		if schemaType == "" {
			schemaType = "AVRO"
		}

		versions = append(versions, SchemaVersion{
			Version:    ver.Version,
			ID:         ver.ID,
			Schema:     ver.Schema,
			SchemaType: schemaType,
		})
	}

	// Get compatibility config: try subject-level first, fall back to global.
	compatibility := c.getCompatibility(ctx, escaped)

	return &SchemaDetail{
		Subject:       subject,
		Compatibility: compatibility,
		Versions:      versions,
	}, nil
}

// CreateSchema registers a new schema version under the given subject.
func (c *Client) CreateSchema(ctx context.Context, req CreateSchemaRequest) (*CreateSchemaResponse, error) {
	schemaType := req.SchemaType
	if schemaType == "" {
		schemaType = "AVRO"
	}

	body := map[string]string{
		"schema":     req.Schema,
		"schemaType": schemaType,
	}

	path := fmt.Sprintf("/subjects/%s/versions", url.PathEscape(req.Subject))

	var resp CreateSchemaResponse
	if err := c.doJSON(ctx, http.MethodPost, path, body, &resp); err != nil {
		return nil, fmt.Errorf("create schema for %q: %w", req.Subject, err)
	}

	return &resp, nil
}

// DeleteSubject deletes a subject and all its versions.
func (c *Client) DeleteSubject(ctx context.Context, subject string) error {
	path := fmt.Sprintf("/subjects/%s", url.PathEscape(subject))

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete subject %q: %w", subject, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("schema registry error (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// getCompatibility tries the subject-level config first, then falls back to global.
func (c *Client) getCompatibility(ctx context.Context, escapedSubject string) string {
	var cfg struct {
		CompatibilityLevel string `json:"compatibilityLevel"`
	}

	// Try subject-level config.
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/config/%s", escapedSubject), nil, &cfg); err == nil && cfg.CompatibilityLevel != "" {
		return cfg.CompatibilityLevel
	}

	// Fall back to global config.
	cfg.CompatibilityLevel = ""
	if err := c.doJSON(ctx, http.MethodGet, "/config", nil, &cfg); err == nil {
		return cfg.CompatibilityLevel
	}

	return ""
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

	req.Header.Set("Accept", "application/vnd.schemaregistry.v1+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")
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
		return fmt.Errorf("schema registry error (%d): %s", resp.StatusCode, string(respBody))
	}

	if dest != nil {
		if err := json.Unmarshal(respBody, dest); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
