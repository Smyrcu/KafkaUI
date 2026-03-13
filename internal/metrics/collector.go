package metrics

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Collector periodically scrapes metrics from all configured endpoints.
type Collector struct {
	store    *Store
	scrapers map[string]*Scraper
	logger   *slog.Logger
	interval time.Duration
}

// NewCollector creates a collector that scrapes each cluster's metrics endpoint.
func NewCollector(store *Store, scrapers map[string]*Scraper, logger *slog.Logger) *Collector {
	return &Collector{
		store:    store,
		scrapers: scrapers,
		logger:   logger,
		interval: 30 * time.Second,
	}
}

// Run starts the collection loop. Blocks until ctx is cancelled.
func (c *Collector) Run(ctx context.Context) {
	c.collect(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collect(ctx)
		}
	}
}

func (c *Collector) collect(ctx context.Context) {
	var wg sync.WaitGroup
	for clusterName, scraper := range c.scrapers {
		wg.Add(1)
		go func(name string, s *Scraper) {
			defer wg.Done()
			snap, err := s.Scrape(ctx)
			if err != nil {
				c.logger.Debug("failed to scrape metrics", "cluster", name, "error", err)
				return
			}
			c.store.Append(name, snap)
		}(clusterName, scraper)
	}
	wg.Wait()
}
