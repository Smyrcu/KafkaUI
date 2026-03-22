package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Smyrcu/KafkaUI/internal/config"
	"github.com/Smyrcu/KafkaUI/internal/kafka"
)

func newAdminTestHandler(t *testing.T, clusters []config.ClusterConfig, staticNames []string) *AdminHandler {
	t.Helper()
	cfg := &config.Config{Clusters: clusters}
	reg, err := kafka.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}
	t.Cleanup(reg.Close)

	tmpDir := t.TempDir()
	dynamicPath := filepath.Join(tmpDir, "dynamic.yaml")
	dynamicCfg := config.NewDynamicConfig(dynamicPath)

	return NewAdminHandler(reg, dynamicCfg, staticNames, nil)
}

func TestAdminHandler_ListClusters(t *testing.T) {
	h := newAdminTestHandler(t, []config.ClusterConfig{
		{Name: "alpha", BootstrapServers: "localhost:9092"},
		{Name: "beta", BootstrapServers: "localhost:9093"},
	}, []string{"alpha", "beta"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clusters", nil)
	rec := httptest.NewRecorder()

	h.ListClusters(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body AdminClusterList
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body.Static) != 2 {
		t.Errorf("expected 2 static clusters, got %d", len(body.Static))
	}
	if len(body.Dynamic) != 0 {
		t.Errorf("expected 0 dynamic clusters, got %d", len(body.Dynamic))
	}
}

func TestAdminHandler_AddCluster(t *testing.T) {
	clusters := []config.ClusterConfig{
		{Name: "alpha", BootstrapServers: "localhost:9092"},
	}
	h := newAdminTestHandler(t, clusters, []string{"alpha"})

	body := `{"name":"gamma","bootstrapServers":"localhost:9094"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clusters?validate=false", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.AddCluster(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	// Verify it's in the registry
	_, ok := h.registry.Get("gamma")
	if !ok {
		t.Fatal("expected to find cluster 'gamma' in registry after add")
	}

	// Verify it's in the dynamic config
	dynClusters, err := h.dynamicCfg.Load()
	if err != nil {
		t.Fatalf("failed to load dynamic config: %v", err)
	}
	if len(dynClusters) != 1 {
		t.Fatalf("expected 1 dynamic cluster, got %d", len(dynClusters))
	}
	if dynClusters[0].Name != "gamma" {
		t.Errorf("expected dynamic cluster name 'gamma', got %q", dynClusters[0].Name)
	}
}

func TestAdminHandler_AddCluster_DuplicateName(t *testing.T) {
	clusters := []config.ClusterConfig{
		{Name: "alpha", BootstrapServers: "localhost:9092"},
	}
	h := newAdminTestHandler(t, clusters, []string{"alpha"})

	body := `{"name":"alpha","bootstrapServers":"localhost:9094"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clusters?validate=false", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.AddCluster(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_DeleteCluster_Static(t *testing.T) {
	clusters := []config.ClusterConfig{
		{Name: "alpha", BootstrapServers: "localhost:9092"},
		{Name: "beta", BootstrapServers: "localhost:9093"},
	}
	h := newAdminTestHandler(t, clusters, []string{"alpha", "beta"})

	r := chi.NewRouter()
	r.Delete("/api/v1/admin/clusters/{name}", h.DeleteCluster)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/clusters/alpha", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_DeleteCluster_LastOne(t *testing.T) {
	// Create a single-cluster registry with a dynamic cluster
	tmpDir := t.TempDir()
	dynamicPath := filepath.Join(tmpDir, "dynamic.yaml")
	dynamicCfg := config.NewDynamicConfig(dynamicPath)

	// Add a dynamic cluster to the config file first
	dc := config.ClusterConfig{Name: "only-one", BootstrapServers: "localhost:9092"}
	if err := dynamicCfg.Add(dc); err != nil {
		t.Fatalf("failed to add dynamic cluster: %v", err)
	}

	cfg := &config.Config{
		Clusters: []config.ClusterConfig{dc},
	}
	reg, err := kafka.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}
	t.Cleanup(reg.Close)

	h := NewAdminHandler(reg, dynamicCfg, nil, nil) // no static names

	r := chi.NewRouter()
	r.Delete("/api/v1/admin/clusters/{name}", h.DeleteCluster)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/clusters/only-one", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_UpdateCluster_Static(t *testing.T) {
	clusters := []config.ClusterConfig{
		{Name: "alpha", BootstrapServers: "localhost:9092"},
	}
	h := newAdminTestHandler(t, clusters, []string{"alpha"})

	r := chi.NewRouter()
	r.Put("/api/v1/admin/clusters/{name}", h.UpdateCluster)

	body := `{"bootstrapServers":"localhost:9999"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/clusters/alpha", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_AddCluster_MissingName(t *testing.T) {
	h := newAdminTestHandler(t, []config.ClusterConfig{
		{Name: "alpha", BootstrapServers: "localhost:9092"},
	}, []string{"alpha"})

	body := `{"bootstrapServers":"localhost:9094"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clusters?validate=false", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.AddCluster(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_AddCluster_MissingBootstrapServers(t *testing.T) {
	h := newAdminTestHandler(t, []config.ClusterConfig{
		{Name: "alpha", BootstrapServers: "localhost:9092"},
	}, []string{"alpha"})

	body := `{"name":"gamma"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clusters?validate=false", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.AddCluster(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

// Verify dynamic clusters are returned in the list after adding.
func TestAdminHandler_ListClusters_WithDynamic(t *testing.T) {
	clusters := []config.ClusterConfig{
		{Name: "alpha", BootstrapServers: "localhost:9092"},
	}
	h := newAdminTestHandler(t, clusters, []string{"alpha"})

	// Add a dynamic cluster
	addBody := `{"name":"dynamic1","bootstrapServers":"localhost:9095"}`
	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/clusters?validate=false", strings.NewReader(addBody))
	addReq.Header.Set("Content-Type", "application/json")
	addRec := httptest.NewRecorder()
	h.AddCluster(addRec, addReq)

	if addRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 on add, got %d", addRec.Code)
	}

	// List and verify
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/clusters", nil)
	listRec := httptest.NewRecorder()
	h.ListClusters(listRec, listReq)

	var body AdminClusterList
	if err := json.NewDecoder(listRec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body.Static) != 1 {
		t.Errorf("expected 1 static cluster, got %d", len(body.Static))
	}
	if len(body.Dynamic) != 1 {
		t.Errorf("expected 1 dynamic cluster, got %d", len(body.Dynamic))
	}
	if body.Dynamic[0].Name != "dynamic1" {
		t.Errorf("expected dynamic cluster name 'dynamic1', got %q", body.Dynamic[0].Name)
	}
}
