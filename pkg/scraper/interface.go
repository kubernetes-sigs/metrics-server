package scraper

import (
	"context"

	"sigs.k8s.io/metrics-server/pkg/storage"
)

type Scraper interface {
	Scrape(ctx context.Context) (*storage.MetricsBatch, error)
}
