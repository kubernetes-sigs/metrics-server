package storage

import "sigs.k8s.io/metrics-server/pkg/api"

type Storage interface {
	api.MetricsGetter
	Store(batch *MetricsBatch)
}
