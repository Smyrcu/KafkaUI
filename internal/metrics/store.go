package metrics

import (
	"sync"
	"time"
)

const maxAge = 24 * time.Hour

// MetricPoint is a single value at a point in time.
type MetricPoint struct {
	Time  time.Time `json:"time"`
	Value float64   `json:"value"`
}

// Store holds per-cluster, per-metric time-series data and the latest snapshot.
type Store struct {
	mu     sync.RWMutex
	series map[string]map[string][]MetricPoint // cluster → metricName → points
	latest map[string]Snapshot                  // cluster → last scrape
}

// NewStore creates an empty metrics store.
func NewStore() *Store {
	return &Store{
		series: make(map[string]map[string][]MetricPoint),
		latest: make(map[string]Snapshot),
	}
}

// Append records a new snapshot: updates latest and appends time-series points.
func (s *Store) Append(cluster string, snap Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.latest[cluster] = snap

	if s.series[cluster] == nil {
		s.series[cluster] = make(map[string][]MetricPoint)
	}
	for name, fam := range snap {
		if len(fam.Samples) > 0 {
			s.series[cluster][name] = append(s.series[cluster][name], MetricPoint{
				Time:  now,
				Value: fam.Samples[0].Value,
			})
		}
	}
	s.evict(cluster)
}

// GetLatest returns the most recent snapshot for a cluster.
func (s *Store) GetLatest(cluster string) (Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.latest[cluster]
	return snap, ok
}

// QueryMetric returns time-series points for a specific metric within the given duration.
func (s *Store) QueryMetric(cluster, metricName string, duration time.Duration) []MetricPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	points := s.series[cluster][metricName]
	if len(points) == 0 {
		return nil
	}

	cutoff := time.Now().Add(-duration)
	lo, hi := 0, len(points)
	for lo < hi {
		mid := (lo + hi) / 2
		if points[mid].Time.Before(cutoff) {
			lo = mid + 1
		} else {
			hi = mid
		}
	}

	result := points[lo:]
	if len(result) == 0 {
		return nil
	}
	out := make([]MetricPoint, len(result))
	copy(out, result)
	return out
}

// HasData returns true if the store has any data for the given cluster.
func (s *Store) HasData(cluster string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.latest[cluster]
	return ok
}

func (s *Store) evict(cluster string) {
	cutoff := time.Now().Add(-maxAge)
	for name, points := range s.series[cluster] {
		lo := 0
		for lo < len(points) && points[lo].Time.Before(cutoff) {
			lo++
		}
		if lo > 0 {
			remaining := make([]MetricPoint, len(points)-lo)
			copy(remaining, points[lo:])
			s.series[cluster][name] = remaining
		}
	}
}
