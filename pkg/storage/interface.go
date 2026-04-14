// Copyright 2020 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"k8s.io/metrics/pkg/apis/metrics"

	"sigs.k8s.io/metrics-server/pkg/api"
)

type Storage interface {
	api.MetricsGetter
	Store(batch *MetricsBatch)
	Ready() bool
}

// WatchableStorage extends Storage with watch capabilities
type WatchableStorage interface {
	Storage

	// CurrentResourceVersion returns the current resource version as a string
	CurrentResourceVersion() string

	// GetAllNodeMetrics returns all currently stored node metrics for initial sync
	GetAllNodeMetrics() []metrics.NodeMetrics

	// GetAllPodMetrics returns all currently stored pod metrics for initial sync
	GetAllPodMetrics() []metrics.PodMetrics

	// RegisterNodeWatcher registers a watcher for node metrics changes
	RegisterNodeWatcher(w MetricsWatcher) uint64

	// UnregisterNodeWatcher removes a node metrics watcher
	UnregisterNodeWatcher(id uint64)

	// RegisterPodWatcher registers a watcher for pod metrics changes
	RegisterPodWatcher(w MetricsWatcher) uint64

	// UnregisterPodWatcher removes a pod metrics watcher
	UnregisterPodWatcher(id uint64)

	// RegisterNodeWatcherWithSnapshot atomically registers a watcher and returns
	// the current snapshot and resource version. This prevents race conditions
	// where Store() could fire between getting the snapshot and registering.
	RegisterNodeWatcherWithSnapshot(w MetricsWatcher) (id uint64, allMetrics []metrics.NodeMetrics, rv string)

	// RegisterPodWatcherWithSnapshot atomically registers a watcher and returns
	// the current snapshot and resource version. This prevents race conditions
	// where Store() could fire between getting the snapshot and registering.
	RegisterPodWatcherWithSnapshot(w MetricsWatcher) (id uint64, allMetrics []metrics.PodMetrics, rv string)

	// Shutdown closes all active watchers. Should be called during server shutdown.
	Shutdown()
}
