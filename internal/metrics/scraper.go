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

type BrokerMetrics struct {
	BytesInPerSec             float64 `json:"bytesInPerSec"`
	BytesOutPerSec            float64 `json:"bytesOutPerSec"`
	MessagesInPerSec          float64 `json:"messagesInPerSec"`
	UnderReplicatedPartitions float64 `json:"underReplicatedPartitions"`
	ActiveControllerCount     float64 `json:"activeControllerCount"`
	OfflinePartitionsCount    float64 `json:"offlinePartitionsCount"`
}

type Scraper struct {
	urlPattern string
	client     *http.Client
}

func NewScraper(urlPattern string) *Scraper {
	return &Scraper{
		urlPattern: urlPattern,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (s *Scraper) buildURL(host string) string {
	return strings.ReplaceAll(s.urlPattern, "{host}", host)
}

func (s *Scraper) Scrape(ctx context.Context, host string) (BrokerMetrics, error) {
	url := s.buildURL(host)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return BrokerMetrics{}, fmt.Errorf("creating request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return BrokerMetrics{}, fmt.Errorf("fetching metrics from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return BrokerMetrics{}, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	return parseMetrics(resp.Body)
}

var metricKeys = map[string]func(*BrokerMetrics, float64){
	"kafka_server_brokertopicmetrics_bytesinpersec":           func(m *BrokerMetrics, v float64) { m.BytesInPerSec = v },
	"kafka_server_brokertopicmetrics_bytesoutpersec":          func(m *BrokerMetrics, v float64) { m.BytesOutPerSec = v },
	"kafka_server_brokertopicmetrics_messagesinpersec":        func(m *BrokerMetrics, v float64) { m.MessagesInPerSec = v },
	"kafka_server_replicamanager_underreplicatedpartitions":   func(m *BrokerMetrics, v float64) { m.UnderReplicatedPartitions = v },
	"kafka_controller_kafkacontroller_activecontrollercount":  func(m *BrokerMetrics, v float64) { m.ActiveControllerCount = v },
	"kafka_controller_kafkacontroller_offlinepartitionscount": func(m *BrokerMetrics, v float64) { m.OfflinePartitionsCount = v },
}

func parseMetrics(r io.Reader) (BrokerMetrics, error) {
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(r)
	if err != nil {
		return BrokerMetrics{}, fmt.Errorf("parsing prometheus metrics: %w", err)
	}

	var m BrokerMetrics
	for name, setter := range metricKeys {
		if fam, ok := families[name]; ok {
			v := extractValue(fam)
			setter(&m, v)
		}
	}
	return m, nil
}

func extractValue(fam *dto.MetricFamily) float64 {
	for _, metric := range fam.GetMetric() {
		if hasEmptyTopicLabel(metric) || len(metric.GetLabel()) == 0 {
			if g := metric.GetGauge(); g != nil {
				return g.GetValue()
			}
			if u := metric.GetUntyped(); u != nil {
				return u.GetValue()
			}
		}
	}
	if len(fam.GetMetric()) > 0 {
		m := fam.GetMetric()[0]
		if g := m.GetGauge(); g != nil {
			return g.GetValue()
		}
		if u := m.GetUntyped(); u != nil {
			return u.GetValue()
		}
	}
	return 0
}

func hasEmptyTopicLabel(m *dto.Metric) bool {
	for _, l := range m.GetLabel() {
		if l.GetName() == "topic" && l.GetValue() == "" {
			return true
		}
	}
	return false
}
