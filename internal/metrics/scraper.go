package metrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

// Sample is a single data point with optional labels.
type Sample struct {
	Labels map[string]string `json:"labels,omitempty"`
	Value  float64           `json:"value"`
}

// MetricFamily is one Prometheus metric with its metadata and all samples.
type MetricFamily struct {
	Name    string   `json:"name"`
	Help    string   `json:"help"`
	Type    string   `json:"type"`
	Samples []Sample `json:"samples"`
}

// Snapshot is the full result of a single scrape — metric name → MetricFamily.
type Snapshot map[string]MetricFamily

// Scraper fetches and parses Prometheus metrics from a URL.
type Scraper struct {
	url    string
	client *http.Client
}

// NewScraper creates a scraper for the given URL (used as-is from config).
func NewScraper(url string) *Scraper {
	return &Scraper{
		url:    url,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Scrape fetches and parses all metrics from the configured URL.
func (s *Scraper) Scrape(ctx context.Context) (Snapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching metrics from %s: %w", s.url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, s.url)
	}

	return parseAllMetrics(resp.Body)
}

func parseAllMetrics(r io.Reader) (Snapshot, error) {
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(r)
	if err != nil {
		return nil, fmt.Errorf("parsing prometheus metrics: %w", err)
	}

	snap := make(Snapshot, len(families))
	for name, fam := range families {
		mf := MetricFamily{
			Name: name,
			Help: fam.GetHelp(),
			Type: strings.ToLower(fam.GetType().String()),
		}
		for _, m := range fam.GetMetric() {
			s := Sample{Value: extractSampleValue(m)}
			if len(m.GetLabel()) > 0 {
				s.Labels = make(map[string]string, len(m.GetLabel()))
				for _, l := range m.GetLabel() {
					s.Labels[l.GetName()] = l.GetValue()
				}
			}
			mf.Samples = append(mf.Samples, s)
		}
		snap[name] = mf
	}
	return snap, nil
}

func extractSampleValue(m *dto.Metric) float64 {
	if g := m.GetGauge(); g != nil {
		return g.GetValue()
	}
	if c := m.GetCounter(); c != nil {
		return c.GetValue()
	}
	if u := m.GetUntyped(); u != nil {
		return u.GetValue()
	}
	return 0
}
