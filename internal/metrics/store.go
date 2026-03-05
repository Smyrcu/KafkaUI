package metrics

import (
	"sync"
	"time"
)

// TimestampedMetrics is a single data point with a timestamp.
type TimestampedMetrics struct {
	Time    time.Time    `json:"time"`
	Metrics BrokerMetrics `json:"metrics"`
}

// Store holds in-memory time-series metrics per broker, keyed by "cluster:brokerID".
// Retains up to 14 days of data at 30s intervals (~40320 points per broker).
type Store struct {
	mu   sync.RWMutex
	data map[string][]TimestampedMetrics
}

const maxAge = 14 * 24 * time.Hour

func NewStore() *Store {
	return &Store{
		data: make(map[string][]TimestampedMetrics),
	}
}

// Append adds a data point for a broker key.
func (s *Store) Append(key string, m BrokerMetrics) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.data[key] = append(s.data[key], TimestampedMetrics{Time: now, Metrics: m})
	s.evict(key)
}

// Query returns data points for a broker key within the given duration from now.
func (s *Store) Query(key string, duration time.Duration) []TimestampedMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	points := s.data[key]
	if len(points) == 0 {
		return nil
	}

	cutoff := time.Now().Add(-duration)
	// Binary search for cutoff
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
	out := make([]TimestampedMetrics, len(result))
	copy(out, result)
	return out
}

func (s *Store) evict(key string) {
	points := s.data[key]
	cutoff := time.Now().Add(-maxAge)
	lo := 0
	for lo < len(points) && points[lo].Time.Before(cutoff) {
		lo++
	}
	if lo > 0 {
		s.data[key] = points[lo:]
	}
}
