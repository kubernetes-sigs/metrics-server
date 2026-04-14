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

package storage

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/metrics/pkg/apis/metrics"

	"sigs.k8s.io/metrics-server/pkg/api"
)

// WatchEvent represents a metrics change event for watchers
type WatchEvent = api.WatchEvent

// MetricsWatcher receives watch events from the storage
type MetricsWatcher = api.MetricsWatcher

// storage is a thread-safe storage for node and pod metrics with watch support.
type storage struct {
	mu    sync.RWMutex
	pods  podStorage
	nodes nodeStorage

	// resourceVersion is a monotonically increasing counter incremented on each Store() call
	resourceVersion atomic.Uint64

	// Watch support
	watchersMu   sync.RWMutex
	nodeWatchers map[uint64]MetricsWatcher
	podWatchers  map[uint64]MetricsWatcher
	watcherID    atomic.Uint64

	// Track previous metrics for diff calculation
	prevNodeMetrics map[string]metrics.NodeMetrics
	prevPodMetrics  map[apitypes.NamespacedName]metrics.PodMetrics
}

var _ Storage = (*storage)(nil)

func NewStorage(metricResolution time.Duration) *storage {
	return &storage{
		pods:            podStorage{metricResolution: metricResolution},
		nodeWatchers:    make(map[uint64]MetricsWatcher),
		podWatchers:     make(map[uint64]MetricsWatcher),
		prevNodeMetrics: make(map[string]metrics.NodeMetrics),
		prevPodMetrics:  make(map[apitypes.NamespacedName]metrics.PodMetrics),
	}
}

// Ready returns true if metrics-server's storage has accumulated enough metric
// points to serve NodeMetrics.
func (s *storage) Ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.nodes.prev) != 0 || len(s.pods.prev) != 0
}

// CurrentResourceVersion returns the current resource version as a string.
func (s *storage) CurrentResourceVersion() string {
	return strconv.FormatUint(s.resourceVersion.Load(), 10)
}

func (s *storage) GetNodeMetrics(nodes ...*corev1.Node) ([]metrics.NodeMetrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nodes.GetMetrics(nodes...)
}

func (s *storage) GetPodMetrics(pods ...*metav1.PartialObjectMetadata) ([]metrics.PodMetrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pods.GetMetrics(pods...)
}

// GetAllNodeMetrics returns all currently stored node metrics.
// Used for initial watch sync.
func (s *storage) GetAllNodeMetrics() []metrics.NodeMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]metrics.NodeMetrics, 0, len(s.prevNodeMetrics))
	for _, m := range s.prevNodeMetrics {
		result = append(result, m)
	}
	return result
}

// GetAllPodMetrics returns all currently stored pod metrics.
// Used for initial watch sync.
func (s *storage) GetAllPodMetrics() []metrics.PodMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]metrics.PodMetrics, 0, len(s.prevPodMetrics))
	for _, m := range s.prevPodMetrics {
		result = append(result, m)
	}
	return result
}

// RegisterNodeWatcher registers a watcher for node metrics changes.
// Returns a watcher ID that can be used to unregister.
func (s *storage) RegisterNodeWatcher(w MetricsWatcher) uint64 {
	id := s.watcherID.Add(1)
	s.watchersMu.Lock()
	s.nodeWatchers[id] = w
	s.watchersMu.Unlock()
	return id
}

// UnregisterNodeWatcher removes a node metrics watcher.
func (s *storage) UnregisterNodeWatcher(id uint64) {
	s.watchersMu.Lock()
	delete(s.nodeWatchers, id)
	s.watchersMu.Unlock()
}

// RegisterPodWatcher registers a watcher for pod metrics changes.
// Returns a watcher ID that can be used to unregister.
func (s *storage) RegisterPodWatcher(w MetricsWatcher) uint64 {
	id := s.watcherID.Add(1)
	s.watchersMu.Lock()
	s.podWatchers[id] = w
	s.watchersMu.Unlock()
	return id
}

// UnregisterPodWatcher removes a pod metrics watcher.
func (s *storage) UnregisterPodWatcher(id uint64) {
	s.watchersMu.Lock()
	delete(s.podWatchers, id)
	s.watchersMu.Unlock()
}

// RegisterNodeWatcherWithSnapshot atomically registers a watcher AND returns
// the current snapshot of node metrics and resource version.
// This ensures no Store() can interleave between getting the snapshot and
// registering the watcher, preventing missed events.
func (s *storage) RegisterNodeWatcherWithSnapshot(w MetricsWatcher) (id uint64, allMetrics []metrics.NodeMetrics, rv string) {
	// Hold both locks to ensure atomicity
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get snapshot while holding the data lock
	allMetrics = make([]metrics.NodeMetrics, 0, len(s.prevNodeMetrics))
	for _, m := range s.prevNodeMetrics {
		allMetrics = append(allMetrics, m)
	}
	rv = strconv.FormatUint(s.resourceVersion.Load(), 10)

	// Register watcher while still holding data lock
	id = s.watcherID.Add(1)
	s.watchersMu.Lock()
	s.nodeWatchers[id] = w
	s.watchersMu.Unlock()

	return id, allMetrics, rv
}

// RegisterPodWatcherWithSnapshot atomically registers a watcher AND returns
// the current snapshot of pod metrics and resource version.
// This ensures no Store() can interleave between getting the snapshot and
// registering the watcher, preventing missed events.
func (s *storage) RegisterPodWatcherWithSnapshot(w MetricsWatcher) (id uint64, allMetrics []metrics.PodMetrics, rv string) {
	// Hold both locks to ensure atomicity
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get snapshot while holding the data lock
	allMetrics = make([]metrics.PodMetrics, 0, len(s.prevPodMetrics))
	for _, m := range s.prevPodMetrics {
		allMetrics = append(allMetrics, m)
	}
	rv = strconv.FormatUint(s.resourceVersion.Load(), 10)

	// Register watcher while still holding data lock
	id = s.watcherID.Add(1)
	s.watchersMu.Lock()
	s.podWatchers[id] = w
	s.watchersMu.Unlock()

	return id, allMetrics, rv
}

func (s *storage) Store(batch *MetricsBatch) {
	var nodeEvents, podEvents []WatchEvent

	// Hold lock only for data operations and event calculation
	s.mu.Lock()

	// Increment resource version
	newRV := s.resourceVersion.Add(1)
	rvStr := strconv.FormatUint(newRV, 10)

	// Store new metrics
	s.nodes.Store(batch)
	s.pods.Store(batch)

	// Calculate events while holding the lock (accesses internal state)
	nodeEvents = s.calculateNodeEvents(rvStr)
	podEvents = s.calculatePodEvents(rvStr)

	s.mu.Unlock() // Release lock BEFORE broadcasting to prevent deadlock

	// Broadcast events outside the data lock
	s.broadcastNodeEvents(nodeEvents)
	s.broadcastPodEvents(podEvents)
}

func (s *storage) calculateNodeEvents(rv string) []WatchEvent {
	var events []WatchEvent

	// Get current node metrics
	currentNodes := make(map[string]metrics.NodeMetrics)
	for nodeName := range s.nodes.prev {
		// We need both last and prev to compute metrics
		if _, hasLast := s.nodes.last[nodeName]; hasLast {
			last := s.nodes.last[nodeName]
			prev := s.nodes.prev[nodeName]
			rl, ti, err := resourceUsage(last, prev)
			if err != nil {
				continue
			}
			m := metrics.NodeMetrics{
				ObjectMeta: metav1.ObjectMeta{
					Name:            nodeName,
					ResourceVersion: rv,
				},
				Timestamp: metav1.NewTime(ti.Timestamp),
				Window:    metav1.Duration{Duration: ti.Window},
				Usage:     rl,
			}
			currentNodes[nodeName] = m
		}
	}

	// Find ADDED and MODIFIED
	for name, current := range currentNodes {
		if _, existed := s.prevNodeMetrics[name]; existed {
			events = append(events, WatchEvent{Type: watch.Modified, Object: current})
		} else {
			events = append(events, WatchEvent{Type: watch.Added, Object: current})
		}
	}

	// Find DELETED
	for name, prev := range s.prevNodeMetrics {
		if _, exists := currentNodes[name]; !exists {
			deleted := prev
			deleted.ResourceVersion = rv
			events = append(events, WatchEvent{Type: watch.Deleted, Object: deleted})
		}
	}

	// Update previous state
	s.prevNodeMetrics = currentNodes

	return events
}

func (s *storage) calculatePodEvents(rv string) []WatchEvent {
	var events []WatchEvent

	// Get current pod metrics
	currentPods := make(map[apitypes.NamespacedName]metrics.PodMetrics)
	for podRef := range s.pods.prev {
		if lastPod, hasLast := s.pods.last[podRef]; hasLast {
			prevPod := s.pods.prev[podRef]

			var (
				cms              = make([]metrics.ContainerMetrics, 0, len(lastPod.Containers))
				earliestTimeInfo api.TimeInfo
			)
			allContainersPresent := true
			for container, lastContainer := range lastPod.Containers {
				prevContainer, found := prevPod.Containers[container]
				if !found {
					allContainersPresent = false
					break
				}
				usage, ti, err := resourceUsage(lastContainer, prevContainer)
				if err != nil {
					continue
				}
				cms = append(cms, metrics.ContainerMetrics{
					Name:  container,
					Usage: usage,
				})
				if earliestTimeInfo.Timestamp.IsZero() || earliestTimeInfo.Timestamp.After(ti.Timestamp) {
					earliestTimeInfo = ti
				}
			}
			if allContainersPresent && len(cms) > 0 {
				m := metrics.PodMetrics{
					ObjectMeta: metav1.ObjectMeta{
						Name:            podRef.Name,
						Namespace:       podRef.Namespace,
						ResourceVersion: rv,
					},
					Timestamp:  metav1.NewTime(earliestTimeInfo.Timestamp),
					Window:     metav1.Duration{Duration: earliestTimeInfo.Window},
					Containers: cms,
				}
				currentPods[podRef] = m
			}
		}
	}

	// Find ADDED and MODIFIED
	for ref, current := range currentPods {
		if _, existed := s.prevPodMetrics[ref]; existed {
			events = append(events, WatchEvent{Type: watch.Modified, Object: current})
		} else {
			events = append(events, WatchEvent{Type: watch.Added, Object: current})
		}
	}

	// Find DELETED
	for ref, prev := range s.prevPodMetrics {
		if _, exists := currentPods[ref]; !exists {
			deleted := prev
			deleted.ResourceVersion = rv
			events = append(events, WatchEvent{Type: watch.Deleted, Object: deleted})
		}
	}

	// Update previous state
	s.prevPodMetrics = currentPods

	return events
}

func (s *storage) broadcastNodeEvents(events []WatchEvent) {
	if len(events) == 0 {
		return
	}

	s.watchersMu.RLock()
	watchers := make([]MetricsWatcher, 0, len(s.nodeWatchers))
	watcherIDs := make([]uint64, 0, len(s.nodeWatchers))
	for id, w := range s.nodeWatchers {
		watchers = append(watchers, w)
		watcherIDs = append(watcherIDs, id)
	}
	s.watchersMu.RUnlock()

	// Send events to each watcher, tracking which ones are closed
	var closedWatchers []uint64
	for i, w := range watchers {
		select {
		case <-w.Done():
			closedWatchers = append(closedWatchers, watcherIDs[i])
			continue
		default:
		}
		for _, event := range events {
			if !w.Send(event) {
				closedWatchers = append(closedWatchers, watcherIDs[i])
				break
			}
		}
	}

	// Clean up closed watchers
	if len(closedWatchers) > 0 {
		s.watchersMu.Lock()
		for _, id := range closedWatchers {
			delete(s.nodeWatchers, id)
		}
		s.watchersMu.Unlock()
	}
}

func (s *storage) broadcastPodEvents(events []WatchEvent) {
	if len(events) == 0 {
		return
	}

	s.watchersMu.RLock()
	watchers := make([]MetricsWatcher, 0, len(s.podWatchers))
	watcherIDs := make([]uint64, 0, len(s.podWatchers))
	for id, w := range s.podWatchers {
		watchers = append(watchers, w)
		watcherIDs = append(watcherIDs, id)
	}
	s.watchersMu.RUnlock()

	// Send events to each watcher, tracking which ones are closed
	var closedWatchers []uint64
	for i, w := range watchers {
		select {
		case <-w.Done():
			closedWatchers = append(closedWatchers, watcherIDs[i])
			continue
		default:
		}
		for _, event := range events {
			if !w.Send(event) {
				closedWatchers = append(closedWatchers, watcherIDs[i])
				break
			}
		}
	}

	// Clean up closed watchers
	if len(closedWatchers) > 0 {
		s.watchersMu.Lock()
		for _, id := range closedWatchers {
			delete(s.podWatchers, id)
		}
		s.watchersMu.Unlock()
	}
}

// Shutdown closes all active watchers. Should be called during server shutdown.
func (s *storage) Shutdown() {
	s.watchersMu.Lock()
	defer s.watchersMu.Unlock()

	// Close all node watchers
	for _, w := range s.nodeWatchers {
		w.Stop()
	}
	s.nodeWatchers = make(map[uint64]MetricsWatcher)

	// Close all pod watchers
	for _, w := range s.podWatchers {
		w.Stop()
	}
	s.podWatchers = make(map[uint64]MetricsWatcher)
}
