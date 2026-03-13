package metrics

import (
	"testing"
	"time"
)

func TestStore_AppendAndGetLatest(t *testing.T) {
	store := NewStore()

	snap := Snapshot{
		"test_metric": MetricFamily{
			Name:    "test_metric",
			Type:    "gauge",
			Samples: []Sample{{Value: 42}},
		},
	}

	store.Append("cluster1", snap)

	latest, ok := store.GetLatest("cluster1")
	if !ok {
		t.Fatal("expected data for cluster1")
	}
	if len(latest) != 1 {
		t.Errorf("expected 1 metric, got %d", len(latest))
	}
	if latest["test_metric"].Samples[0].Value != 42 {
		t.Errorf("expected value 42, got %f", latest["test_metric"].Samples[0].Value)
	}
}

func TestStore_GetLatest_Missing(t *testing.T) {
	store := NewStore()

	_, ok := store.GetLatest("nonexistent")
	if ok {
		t.Fatal("expected no data for nonexistent cluster")
	}
}

func TestStore_HasData(t *testing.T) {
	store := NewStore()

	if store.HasData("cluster1") {
		t.Fatal("expected no data initially")
	}

	store.Append("cluster1", Snapshot{
		"m": MetricFamily{Name: "m", Samples: []Sample{{Value: 1}}},
	})

	if !store.HasData("cluster1") {
		t.Fatal("expected data after append")
	}
}

func TestStore_QueryMetric(t *testing.T) {
	store := NewStore()

	// Append two snapshots
	store.Append("c1", Snapshot{
		"metric_a": MetricFamily{Name: "metric_a", Samples: []Sample{{Value: 10}}},
	})
	store.Append("c1", Snapshot{
		"metric_a": MetricFamily{Name: "metric_a", Samples: []Sample{{Value: 20}}},
	})

	points := store.QueryMetric("c1", "metric_a", time.Hour)
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
	if points[0].Value != 10 {
		t.Errorf("expected first value 10, got %f", points[0].Value)
	}
	if points[1].Value != 20 {
		t.Errorf("expected second value 20, got %f", points[1].Value)
	}
}

func TestStore_QueryMetric_Empty(t *testing.T) {
	store := NewStore()

	points := store.QueryMetric("nonexistent", "metric", time.Hour)
	if points != nil {
		t.Errorf("expected nil for nonexistent, got %v", points)
	}
}

func TestStore_LatestOverwrite(t *testing.T) {
	store := NewStore()

	store.Append("c1", Snapshot{
		"m": MetricFamily{Name: "m", Samples: []Sample{{Value: 1}}},
	})
	store.Append("c1", Snapshot{
		"m": MetricFamily{Name: "m", Samples: []Sample{{Value: 99}}},
	})

	latest, ok := store.GetLatest("c1")
	if !ok {
		t.Fatal("expected data")
	}
	if latest["m"].Samples[0].Value != 99 {
		t.Errorf("expected latest value 99, got %f", latest["m"].Samples[0].Value)
	}
}
