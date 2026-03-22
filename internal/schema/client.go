package schema

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/Smyrcu/KafkaUI/internal/httpclient"
)

type Client struct {
	http        *httpclient.Client
	schemaCache sync.Map // schema ID (int) -> schema JSON (string); immutable, never expires
	fetchGroup  singleflight.Group
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
		http: httpclient.New(
			strings.TrimRight(baseURL, "/"),
			10*time.Second,
			"application/vnd.schemaregistry.v1+json",
			"application/vnd.schemaregistry.v1+json",
			"schema registry error",
		),
	}
}

// ListSubjects returns all subjects with their latest version info.
func (c *Client) ListSubjects(ctx context.Context) ([]SubjectInfo, error) {
	var subjects []string
	if err := c.http.Do(ctx, "GET", "/subjects", nil, &subjects); err != nil {
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
		if err := c.http.Do(ctx, "GET", path, nil, &latest); err != nil {
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
	if err := c.http.Do(ctx, "GET", fmt.Sprintf("/subjects/%s/versions", escaped), nil, &versionNums); err != nil {
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
		if err := c.http.Do(ctx, "GET", path, nil, &ver); err != nil {
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
	if err := c.http.Do(ctx, "POST", path, body, &resp); err != nil {
		return nil, fmt.Errorf("create schema for %q: %w", req.Subject, err)
	}

	return &resp, nil
}

// DeleteSubject deletes a subject and all its versions.
func (c *Client) DeleteSubject(ctx context.Context, subject string) error {
	path := fmt.Sprintf("/subjects/%s", url.PathEscape(subject))

	if _, err := c.http.DoRaw(ctx, "DELETE", path, nil); err != nil {
		return fmt.Errorf("delete subject %q: %w", subject, err)
	}

	return nil
}

// getCompatibility tries the subject-level config first, then falls back to global.
func (c *Client) getCompatibility(ctx context.Context, escapedSubject string) string {
	var cfg struct {
		CompatibilityLevel string `json:"compatibilityLevel"`
	}

	// Try subject-level config.
	if err := c.http.Do(ctx, "GET", fmt.Sprintf("/config/%s", escapedSubject), nil, &cfg); err == nil && cfg.CompatibilityLevel != "" {
		return cfg.CompatibilityLevel
	}

	// Fall back to global config.
	cfg.CompatibilityLevel = ""
	if err := c.http.Do(ctx, "GET", "/config", nil, &cfg); err == nil {
		return cfg.CompatibilityLevel
	}

	return ""
}

// GetSchemaByID fetches a schema by its global ID. Results are cached
// indefinitely because schema IDs are immutable in Confluent Schema Registry.
func (c *Client) GetSchemaByID(ctx context.Context, id int) (string, error) {
	if cached, ok := c.schemaCache.Load(id); ok {
		return cached.(string), nil
	}

	key := fmt.Sprintf("schema-%d", id)
	result, err, _ := c.fetchGroup.Do(key, func() (any, error) {
		var resp struct {
			Schema string `json:"schema"`
		}
		if err := c.http.Do(ctx, "GET", fmt.Sprintf("/schemas/ids/%d", id), nil, &resp); err != nil {
			return "", fmt.Errorf("get schema by id %d: %w", id, err)
		}
		c.schemaCache.Store(id, resp.Schema)
		return resp.Schema, nil
	})
	if err != nil {
		return "", err
	}
	return result.(string), nil
}
