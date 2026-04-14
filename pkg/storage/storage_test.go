// Copyright 2021 The Kubernetes Authors.
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
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	MiByte     = 1024 * 1024
	CoreSecond = 1000 * 1000 * 1000
)

func newMetricsPoint(st time.Time, ts time.Time, cpu, memory uint64) MetricsPoint {
	return MetricsPoint{
		StartTime:         st,
		Timestamp:         ts,
		CumulativeCPUUsed: cpu,
		MemoryUsage:       memory,
	}
}

func TestStorage_ResourceVersionIncrement(t *testing.T) {
	s := NewStorage(15 * time.Second)

	if s.CurrentResourceVersion() != "0" {
		t.Errorf("Initial resource version should be 0, got %s", s.CurrentResourceVersion())
	}

	// Store an empty batch
	s.Store(&MetricsBatch{
		Nodes: map[string]MetricsPoint{},
		Pods:  map[types.NamespacedName]PodMetricsPoint{},
	})

	if s.CurrentResourceVersion() != "1" {
		t.Errorf("Resource version should be 1 after first store, got %s", s.CurrentResourceVersion())
	}

	// Store again
	s.Store(&MetricsBatch{
		Nodes: map[string]MetricsPoint{},
		Pods:  map[types.NamespacedName]PodMetricsPoint{},
	})

	if s.CurrentResourceVersion() != "2" {
		t.Errorf("Resource version should be 2 after second store, got %s", s.CurrentResourceVersion())
	}
}

func TestStorage_WatcherRegistration(t *testing.T) {
	s := NewStorage(15 * time.Second)
	watcher := &fakeWatcher{done: make(chan struct{})}

	// Register node watcher
	id := s.RegisterNodeWatcher(watcher)
	if id == 0 {
		t.Error("Watcher ID should be non-zero")
	}

	// Unregister
	s.UnregisterNodeWatcher(id)

	// Register pod watcher
	id2 := s.RegisterPodWatcher(watcher)
	if id2 <= id {
		t.Error("Watcher IDs should be increasing")
	}

	s.UnregisterPodWatcher(id2)
}

func TestStorage_NodeWatchEvents(t *testing.T) {
	s := NewStorage(15 * time.Second)

	watcher := &fakeWatcher{
		done:   make(chan struct{}),
		events: make([]WatchEvent, 0),
	}

	s.RegisterNodeWatcher(watcher)

	now := time.Now()

	// First store - creates node1 (but no metrics yet - need prev)
	s.Store(&MetricsBatch{
		Nodes: map[string]MetricsPoint{
			"node1": newMetricsPoint(now.Add(-time.Minute), now.Add(-30*time.Second), 1*CoreSecond, 100*MiByte),
		},
		Pods: map[types.NamespacedName]PodMetricsPoint{},
	})

	// Second store - now we have prev and last, should emit events
	s.Store(&MetricsBatch{
		Nodes: map[string]MetricsPoint{
			"node1": newMetricsPoint(now.Add(-time.Minute), now, 2*CoreSecond, 100*MiByte),
		},
		Pods: map[types.NamespacedName]PodMetricsPoint{},
	})

	watcher.mu.Lock()
	events := watcher.events
	watcher.mu.Unlock()

	// We should have received at least one event (ADDED for node1)
	if len(events) == 0 {
		t.Error("Expected at least one watch event")
	}

	// First event should be ADDED since node1 wasn't tracked before
	found := false
	for _, e := range events {
		if e.Type == watch.Added || e.Type == watch.Modified {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected ADDED or MODIFIED event for node1")
	}
}

func TestStorage_NodeWatchDeleteEvent(t *testing.T) {
	s := NewStorage(15 * time.Second)

	watcher := &fakeWatcher{
		done:   make(chan struct{}),
		events: make([]WatchEvent, 0),
	}

	now := time.Now()

	// First store - establishes prev
	s.Store(&MetricsBatch{
		Nodes: map[string]MetricsPoint{
			"node1": newMetricsPoint(now.Add(-time.Minute), now.Add(-30*time.Second), 1*CoreSecond, 100*MiByte),
		},
		Pods: map[types.NamespacedName]PodMetricsPoint{},
	})

	// Second store - establishes metrics
	s.Store(&MetricsBatch{
		Nodes: map[string]MetricsPoint{
			"node1": newMetricsPoint(now.Add(-time.Minute), now, 2*CoreSecond, 100*MiByte),
		},
		Pods: map[types.NamespacedName]PodMetricsPoint{},
	})

	// Now register watcher
	s.RegisterNodeWatcher(watcher)

	// Third store - node1 disappears
	s.Store(&MetricsBatch{
		Nodes: map[string]MetricsPoint{},
		Pods:  map[types.NamespacedName]PodMetricsPoint{},
	})

	watcher.mu.Lock()
	events := watcher.events
	watcher.mu.Unlock()

	// Should have a DELETED event
	found := false
	for _, e := range events {
		if e.Type == watch.Deleted {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected DELETED event for removed node")
	}
}

func TestStorage_GetAllNodeMetrics(t *testing.T) {
	s := NewStorage(15 * time.Second)

	now := time.Now()

	// First store
	s.Store(&MetricsBatch{
		Nodes: map[string]MetricsPoint{
			"node1": newMetricsPoint(now.Add(-time.Minute), now.Add(-30*time.Second), 1*CoreSecond, 100*MiByte),
			"node2": newMetricsPoint(now.Add(-time.Minute), now.Add(-30*time.Second), 2*CoreSecond, 200*MiByte),
		},
		Pods: map[types.NamespacedName]PodMetricsPoint{},
	})

	// Second store - establishes metrics
	s.Store(&MetricsBatch{
		Nodes: map[string]MetricsPoint{
			"node1": newMetricsPoint(now.Add(-time.Minute), now, 2*CoreSecond, 100*MiByte),
			"node2": newMetricsPoint(now.Add(-time.Minute), now, 4*CoreSecond, 200*MiByte),
		},
		Pods: map[types.NamespacedName]PodMetricsPoint{},
	})

	allNodes := s.GetAllNodeMetrics()
	if len(allNodes) != 2 {
		t.Errorf("Expected 2 node metrics, got %d", len(allNodes))
	}
}

func TestStorage_GetAllPodMetrics(t *testing.T) {
	s := NewStorage(15 * time.Second)

	now := time.Now()
	podRef := types.NamespacedName{Name: "pod1", Namespace: "default"}

	// First store
	s.Store(&MetricsBatch{
		Nodes: map[string]MetricsPoint{},
		Pods: map[types.NamespacedName]PodMetricsPoint{
			podRef: {
				Containers: map[string]MetricsPoint{
					"container1": newMetricsPoint(now.Add(-time.Minute), now.Add(-30*time.Second), 1*CoreSecond, 50*MiByte),
				},
			},
		},
	})

	// Second store - establishes metrics
	s.Store(&MetricsBatch{
		Nodes: map[string]MetricsPoint{},
		Pods: map[types.NamespacedName]PodMetricsPoint{
			podRef: {
				Containers: map[string]MetricsPoint{
					"container1": newMetricsPoint(now.Add(-time.Minute), now, 2*CoreSecond, 50*MiByte),
				},
			},
		},
	})

	allPods := s.GetAllPodMetrics()
	if len(allPods) != 1 {
		t.Errorf("Expected 1 pod metrics, got %d", len(allPods))
	}
}

func TestStorage_PodWatchDeleteEvent(t *testing.T) {
	s := NewStorage(15 * time.Second)

	now := time.Now()
	podRef := types.NamespacedName{Namespace: "ns1", Name: "pod1"}

	watcher := &fakeWatcher{done: make(chan struct{})}

	// First store - establishes prev
	s.Store(&MetricsBatch{
		Nodes: map[string]MetricsPoint{},
		Pods: map[types.NamespacedName]PodMetricsPoint{
			podRef: {
				Containers: map[string]MetricsPoint{
					"container1": newMetricsPoint(now.Add(-time.Minute), now.Add(-30*time.Second), 1*CoreSecond, 50*MiByte),
				},
			},
		},
	})

	// Second store - establishes metrics
	s.Store(&MetricsBatch{
		Nodes: map[string]MetricsPoint{},
		Pods: map[types.NamespacedName]PodMetricsPoint{
			podRef: {
				Containers: map[string]MetricsPoint{
					"container1": newMetricsPoint(now.Add(-time.Minute), now, 2*CoreSecond, 50*MiByte),
				},
			},
		},
	})

	// Register watcher
	s.RegisterPodWatcher(watcher)

	// Third store - pod disappears
	s.Store(&MetricsBatch{
		Nodes: map[string]MetricsPoint{},
		Pods:  map[types.NamespacedName]PodMetricsPoint{},
	})

	watcher.mu.Lock()
	events := watcher.events
	watcher.mu.Unlock()

	// Should have a DELETED event
	found := false
	for _, e := range events {
		if e.Type == watch.Deleted {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected DELETED event for removed pod")
	}
}

func TestStorage_ConcurrentStoreWatch(t *testing.T) {
	s := NewStorage(15 * time.Second)

	now := time.Now()

	// Initial stores to establish metrics
	s.Store(&MetricsBatch{
		Nodes: map[string]MetricsPoint{
			"node1": newMetricsPoint(now.Add(-time.Minute), now.Add(-30*time.Second), 1*CoreSecond, 100*MiByte),
		},
		Pods: map[types.NamespacedName]PodMetricsPoint{},
	})

	s.Store(&MetricsBatch{
		Nodes: map[string]MetricsPoint{
			"node1": newMetricsPoint(now.Add(-time.Minute), now, 2*CoreSecond, 100*MiByte),
		},
		Pods: map[types.NamespacedName]PodMetricsPoint{},
	})

	var wg sync.WaitGroup
	const numWatchers = 10
	const numStores = 100

	watchers := make([]*fakeWatcher, numWatchers)
	for i := 0; i < numWatchers; i++ {
		watchers[i] = &fakeWatcher{done: make(chan struct{})}
	}

	// Start watcher registrations concurrently
	for i := 0; i < numWatchers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s.RegisterNodeWatcher(watchers[idx])
		}(i)
	}

	// Start store operations concurrently
	for i := 0; i < numStores; i++ {
		wg.Add(1)
		go func(iter int) {
			defer wg.Done()
			s.Store(&MetricsBatch{
				Nodes: map[string]MetricsPoint{
					"node1": newMetricsPoint(now.Add(-time.Minute), now.Add(time.Duration(iter)*time.Second), uint64((iter+3)*CoreSecond), 100*MiByte),
				},
				Pods: map[types.NamespacedName]PodMetricsPoint{},
			})
		}(i)
	}

	wg.Wait()

	// Verify no panic occurred and watchers received events
	// (The race detector will catch data races)
	totalEvents := 0
	for _, w := range watchers {
		w.mu.Lock()
		totalEvents += len(w.events)
		w.mu.Unlock()
	}

	// Watchers registered concurrently, so they may have received varying numbers of events
	// Just verify the test didn't panic and the race detector passed
	t.Logf("Total events received across %d watchers: %d", numWatchers, totalEvents)
}

// fakeWatcher implements MetricsWatcher for testing
type fakeWatcher struct {
	mu     sync.Mutex
	done   chan struct{}
	events []WatchEvent
}

func (f *fakeWatcher) Send(event WatchEvent) bool {
	select {
	case <-f.done:
		return false
	default:
	}
	f.mu.Lock()
	f.events = append(f.events, event)
	f.mu.Unlock()
	return true
}

func (f *fakeWatcher) Done() <-chan struct{} {
	return f.done
}

func (f *fakeWatcher) Stop() {
	select {
	case <-f.done:
		return
	default:
		close(f.done)
	}
}
