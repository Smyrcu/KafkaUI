package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// BrokerInfo is the minimal broker info needed for scraping.
type BrokerInfo struct {
	ID   int32
	Host string
}

// BrokerLister returns broker addresses for scraping.
type BrokerLister func(ctx context.Context) ([]BrokerInfo, error)

// Collector periodically scrapes metrics from all brokers and stores them.
type Collector struct {
	store    *Store
	scrapers map[string]*Scraper
	listers  map[string]BrokerLister
	logger   *slog.Logger
	interval time.Duration
}

func NewCollector(store *Store, scrapers map[string]*Scraper, listers map[string]BrokerLister, logger *slog.Logger) *Collector {
	return &Collector{
		store:    store,
		scrapers: scrapers,
		listers:  listers,
		logger:   logger,
		interval: 5 * time.Second,
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
	for clusterName, scraper := range c.scrapers {
		lister, ok := c.listers[clusterName]
		if !ok {
			continue
		}

		brokers, err := lister(ctx)
		if err != nil {
			c.logger.Debug("failed to list brokers for metrics", "cluster", clusterName, "error", err)
			continue
		}

		var wg sync.WaitGroup
		for _, b := range brokers {
			wg.Add(1)
			go func(broker BrokerInfo) {
				defer wg.Done()
				m, err := scraper.Scrape(ctx, broker.Host)
				if err != nil {
					c.logger.Debug("failed to scrape broker metrics", "cluster", clusterName, "broker", broker.ID, "error", err)
					return
				}
				key := fmt.Sprintf("%s:%d", clusterName, broker.ID)
				c.store.Append(key, m)
			}(b)
		}
		wg.Wait()
	}
}
