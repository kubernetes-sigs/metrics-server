// Copyright 2018 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/metrics/pkg/apis/metrics"
)

// MetricsGetter is both a PodMetricsGetter and a NodeMetricsGetter
type MetricsGetter interface {
	PodMetricsGetter
	NodeMetricsGetter
}

// TimeInfo represents the timing information for a metric, which was
// potentially calculated over some window of time (e.g. for CPU usage rate).
type TimeInfo struct {
	// NB: we consider the earliest timestamp amongst multiple containers
	// for the purposes of determining if a metric is tained by a time
	// period, like pod startup (used by things like the HPA).

	// Timestamp is the time at which the metrics were initially collected.
	// In the case of a rate metric, it should be the timestamp of the last
	// data point used in the calculation.  If it represents multiple metric
	// points, it should be the earliest such timestamp from all of the points.
	Timestamp time.Time

	// Window represents the window used to calculate rate metrics associated
	// with this timestamp.
	Window time.Duration
}

// PodMetricsGetter knows how to fetch metrics for the containers in a pod.
type PodMetricsGetter interface {
	// GetPodMetrics gets the latest metrics for all containers in each listed pod,
	// returning both the metrics and the associated collection timestamp.
	GetPodMetrics(pods ...*metav1.PartialObjectMetadata) ([]metrics.PodMetrics, error)
}

// NodeMetricsGetter knows how to fetch metrics for a node.
type NodeMetricsGetter interface {
	// GetNodeMetrics gets the latest metrics for the given nodes,
	// returning both the metrics and the associated collection timestamp.
	GetNodeMetrics(nodes ...*corev1.Node) ([]metrics.NodeMetrics, error)
}

// WatchEvent represents a metrics change event
type WatchEvent struct {
	Type   watch.EventType
	Object interface{} // Either metrics.NodeMetrics or metrics.PodMetrics
}

// MetricsWatcher receives watch events from the storage
type MetricsWatcher interface {
	// Send sends an event to the watcher. Returns false if the watcher is closed or full.
	Send(event WatchEvent) bool
	// Done returns a channel that's closed when the watcher is stopped
	Done() <-chan struct{}
	// Stop closes the watcher
	Stop()
}

// WatchablePodMetricsGetter extends PodMetricsGetter with watch support
type WatchablePodMetricsGetter interface {
	PodMetricsGetter

	// CurrentResourceVersion returns the current resource version
	CurrentResourceVersion() string

	// GetAllPodMetrics returns all currently stored pod metrics for initial sync
	GetAllPodMetrics() []metrics.PodMetrics

	// RegisterPodWatcher registers a watcher for pod metrics changes
	RegisterPodWatcher(w MetricsWatcher) uint64

	// UnregisterPodWatcher removes a pod metrics watcher
	UnregisterPodWatcher(id uint64)

	// RegisterPodWatcherWithSnapshot atomically registers a watcher and returns
	// the current snapshot and resource version. This prevents race conditions.
	RegisterPodWatcherWithSnapshot(w MetricsWatcher) (id uint64, allMetrics []metrics.PodMetrics, rv string)
}

// WatchableNodeMetricsGetter extends NodeMetricsGetter with watch support
type WatchableNodeMetricsGetter interface {
	NodeMetricsGetter

	// CurrentResourceVersion returns the current resource version
	CurrentResourceVersion() string

	// GetAllNodeMetrics returns all currently stored node metrics for initial sync
	GetAllNodeMetrics() []metrics.NodeMetrics

	// RegisterNodeWatcher registers a watcher for node metrics changes
	RegisterNodeWatcher(w MetricsWatcher) uint64

	// UnregisterNodeWatcher removes a node metrics watcher
	UnregisterNodeWatcher(id uint64)

	// RegisterNodeWatcherWithSnapshot atomically registers a watcher and returns
	// the current snapshot and resource version. This prevents race conditions.
	RegisterNodeWatcherWithSnapshot(w MetricsWatcher) (id uint64, allMetrics []metrics.NodeMetrics, rv string)
}
