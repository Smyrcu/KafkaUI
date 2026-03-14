package metrics

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCollector_CollectsFromScrapers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "# HELP test_metric A test\n# TYPE test_metric gauge\ntest_metric 42\n")
	}))
	defer srv.Close()

	store := NewStore()
	scrapers := map[string]*Scraper{
		"cluster1": NewScraper(srv.URL),
	}
	c := NewCollector(store, scrapers, testLogger())

	c.collect(context.Background())

	if !store.HasData("cluster1") {
		t.Fatal("expected store to have data for cluster1")
	}

	snap, ok := store.GetLatest("cluster1")
	if !ok {
		t.Fatal("expected GetLatest to return data for cluster1")
	}

	fam, ok := snap["test_metric"]
	if !ok {
		t.Fatal("expected snapshot to contain test_metric")
	}
	if fam.Type != "gauge" {
		t.Errorf("expected type gauge, got %s", fam.Type)
	}
	if len(fam.Samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(fam.Samples))
	}
	if fam.Samples[0].Value != 42 {
		t.Errorf("expected value 42, got %f", fam.Samples[0].Value)
	}
}

func TestCollector_HandlesScraperError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	store := NewStore()
	scrapers := map[string]*Scraper{
		"failing": NewScraper(srv.URL),
	}
	c := NewCollector(store, scrapers, testLogger())

	c.collect(context.Background())

	if store.HasData("failing") {
		t.Fatal("expected store to have no data for failing cluster")
	}
}

func TestCollector_RunStopsOnCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "# HELP up Up\n# TYPE up gauge\nup 1\n")
	}))
	defer srv.Close()

	store := NewStore()
	scrapers := map[string]*Scraper{
		"cluster1": NewScraper(srv.URL),
	}
	c := NewCollector(store, scrapers, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		c.Run(ctx)
		close(done)
	}()

	// Give Run a moment to start and perform the initial collect.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Run exited as expected.
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not stop within 3 seconds after context cancellation")
	}
}

func TestCollector_MultipleClustersConcurrent(t *testing.T) {
	makeServer := func(name string, value float64) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "# HELP %s_metric Metric for %s\n# TYPE %s_metric gauge\n%s_metric %g\n",
				name, name, name, name, value)
		}))
	}

	srv1 := makeServer("alpha", 10)
	defer srv1.Close()
	srv2 := makeServer("beta", 20)
	defer srv2.Close()
	srv3 := makeServer("gamma", 30)
	defer srv3.Close()

	store := NewStore()
	scrapers := map[string]*Scraper{
		"cluster-a": NewScraper(srv1.URL),
		"cluster-b": NewScraper(srv2.URL),
		"cluster-c": NewScraper(srv3.URL),
	}
	c := NewCollector(store, scrapers, testLogger())

	c.collect(context.Background())

	for _, cluster := range []string{"cluster-a", "cluster-b", "cluster-c"} {
		if !store.HasData(cluster) {
			t.Errorf("expected store to have data for %s", cluster)
		}
	}

	// Verify specific values per cluster.
	checks := map[string]struct {
		metric string
		value  float64
	}{
		"cluster-a": {"alpha_metric", 10},
		"cluster-b": {"beta_metric", 20},
		"cluster-c": {"gamma_metric", 30},
	}

	for cluster, check := range checks {
		snap, ok := store.GetLatest(cluster)
		if !ok {
			t.Fatalf("expected GetLatest to return data for %s", cluster)
		}
		fam, ok := snap[check.metric]
		if !ok {
			t.Errorf("expected snapshot for %s to contain %s", cluster, check.metric)
			continue
		}
		if len(fam.Samples) == 0 {
			t.Errorf("expected at least 1 sample for %s/%s", cluster, check.metric)
			continue
		}
		if fam.Samples[0].Value != check.value {
			t.Errorf("expected %s/%s value %g, got %g", cluster, check.metric, check.value, fam.Samples[0].Value)
		}
	}
}
