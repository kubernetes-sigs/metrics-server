// Copyright 2024 The Kubernetes Authors.
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
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/metrics/pkg/apis/metrics"
)

func TestMetricsWatcher_BasicSendReceive(t *testing.T) {
	w := newMetricsWatcher("", labels.Everything(), resourceTypePod, nil)
	defer w.Stop()

	// Send an event
	event := WatchEvent{
		Type:   watch.Added,
		Object: metrics.NodeMetrics{},
	}
	if !w.Send(event) {
		t.Fatal("Send returned false for open watcher")
	}

	// Receive the event
	select {
	case received := <-w.ResultChan():
		if received.Type != watch.Added {
			t.Errorf("Expected Added event, got %v", received.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

func TestMetricsWatcher_StopClosesChannel(t *testing.T) {
	w := newMetricsWatcher("", labels.Everything(), resourceTypePod, nil)

	// Stop the watcher
	w.Stop()

	// Send should return false
	event := WatchEvent{
		Type:   watch.Added,
		Object: metrics.NodeMetrics{},
	}
	if w.Send(event) {
		t.Error("Send should return false after Stop")
	}

	// Done channel should be closed
	select {
	case <-w.Done():
		// Expected
	default:
		t.Error("Done channel should be closed after Stop")
	}
}

func TestMetricsWatcher_NamespaceFilter(t *testing.T) {
	w := newMetricsWatcher("test-ns", labels.Everything(), resourceTypePod, nil)
	defer w.Stop()

	// Event in matching namespace
	matchingPod := metrics.PodMetrics{}
	matchingPod.Namespace = "test-ns"
	matchingPod.Name = "pod1"

	// Event in different namespace
	nonMatchingPod := metrics.PodMetrics{}
	nonMatchingPod.Namespace = "other-ns"
	nonMatchingPod.Name = "pod2"

	// Send both
	w.Send(WatchEvent{Type: watch.Added, Object: matchingPod})
	w.Send(WatchEvent{Type: watch.Added, Object: nonMatchingPod})

	// Should only receive the matching one
	select {
	case received := <-w.ResultChan():
		if received.Object.(*metrics.PodMetrics).Name != "pod1" {
			t.Errorf("Expected pod1, got %v", received.Object.(*metrics.PodMetrics).Name)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event")
	}

	// Should not receive any more events
	select {
	case received := <-w.ResultChan():
		t.Errorf("Unexpected event received: %v", received)
	case <-time.After(100 * time.Millisecond):
		// Expected - no more events
	}
}

func TestMetricsWatcher_LabelSelectorFilter(t *testing.T) {
	selector, _ := labels.Parse("app=myapp")
	w := newMetricsWatcher("", selector, resourceTypeNode, nil)
	defer w.Stop()

	// Event with matching labels
	matchingNode := metrics.NodeMetrics{}
	matchingNode.Name = "node1"
	matchingNode.Labels = map[string]string{"app": "myapp"}

	// Event with non-matching labels
	nonMatchingNode := metrics.NodeMetrics{}
	nonMatchingNode.Name = "node2"
	nonMatchingNode.Labels = map[string]string{"app": "other"}

	// Send both
	w.Send(WatchEvent{Type: watch.Added, Object: matchingNode})
	w.Send(WatchEvent{Type: watch.Added, Object: nonMatchingNode})

	// Should only receive the matching one
	select {
	case received := <-w.ResultChan():
		if received.Object.(*metrics.NodeMetrics).Name != "node1" {
			t.Errorf("Expected node1, got %v", received.Object.(*metrics.NodeMetrics).Name)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event")
	}

	// Should not receive any more events
	select {
	case received := <-w.ResultChan():
		t.Errorf("Unexpected event received: %v", received)
	case <-time.After(100 * time.Millisecond):
		// Expected
	}
}

func TestMetricsWatcher_SendInitialEvents(t *testing.T) {
	w := newMetricsWatcher("", labels.Everything(), resourceTypePod, nil)
	defer w.Stop()

	// Create some initial objects
	objects := []interface{}{
		&metrics.NodeMetrics{},
		&metrics.NodeMetrics{},
	}
	objects[0].(*metrics.NodeMetrics).Name = "node1"
	objects[1].(*metrics.NodeMetrics).Name = "node2"

	// Convert to runtime.Object slice
	runtimeObjects := make([]interface{}, len(objects))
	for i, obj := range objects {
		runtimeObjects[i] = obj
	}

	// Send initial events manually (testing the internal function)
	for _, obj := range objects {
		w.result <- watch.Event{Type: watch.Added, Object: obj.(*metrics.NodeMetrics)}
	}
	// Send bookmark
	bookmark := &metrics.PodMetrics{}
	bookmark.ResourceVersion = "123"
	w.result <- watch.Event{Type: watch.Bookmark, Object: bookmark}

	// Should receive 2 ADDED events + 1 BOOKMARK
	receivedCount := 0
	bookmarkReceived := false

	for i := 0; i < 3; i++ {
		select {
		case received := <-w.ResultChan():
			if received.Type == watch.Added {
				receivedCount++
			} else if received.Type == watch.Bookmark {
				bookmarkReceived = true
			}
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for event")
		}
	}

	if receivedCount != 2 {
		t.Errorf("Expected 2 Added events, got %d", receivedCount)
	}
	if !bookmarkReceived {
		t.Error("Expected bookmark event")
	}
}

func TestMetricsWatcher_SlowConsumer(t *testing.T) {
	w := newMetricsWatcher("", labels.Everything(), resourceTypePod, nil)
	defer w.Stop()

	event := WatchEvent{
		Type:   watch.Added,
		Object: metrics.NodeMetrics{},
	}

	// Fill the buffer
	for i := 0; i < watcherBufferSize; i++ {
		if !w.Send(event) {
			t.Fatalf("Send failed at iteration %d", i)
		}
	}

	// The next send should fail (buffer full, slow consumer)
	if w.Send(event) {
		t.Error("Send should fail when buffer is full")
	}

	// Done should be closed
	select {
	case <-w.Done():
		// Expected
	default:
		t.Error("Watcher should be stopped after buffer overflow")
	}
}

func TestMetricsWatcher_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	storage := &fakeWatchablePodStorage{
		currentRV:  "1",
		allMetrics: []metrics.PodMetrics{},
	}

	helper := NewPodMetricsWatchHelper(storage, nil)

	w, err := helper.Watch(ctx, "", labels.Everything(), false)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	// Cast to access Done() channel
	mw, ok := w.(*metricsWatcher)
	if !ok {
		t.Fatal("Expected metricsWatcher type")
	}

	// Cancel the context
	cancel()

	// Give time for cleanup goroutine
	time.Sleep(100 * time.Millisecond)

	// Watcher should be stopped - check the Done channel
	select {
	case <-mw.Done():
		// Expected - watcher is stopped
	case <-time.After(time.Second):
		t.Error("Watcher Done channel should be closed after context cancellation")
	}
}

func TestPodMetricsWatchHelper_WatchWithInitialEvents(t *testing.T) {
	pod1 := metrics.PodMetrics{}
	pod1.Name = "pod1"
	pod1.Namespace = "default"

	pod2 := metrics.PodMetrics{}
	pod2.Name = "pod2"
	pod2.Namespace = "default"

	storage := &fakeWatchablePodStorage{
		currentRV:  "42",
		allMetrics: []metrics.PodMetrics{pod1, pod2},
	}

	helper := NewPodMetricsWatchHelper(storage, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := helper.Watch(ctx, "", labels.Everything(), true)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}
	defer w.Stop()

	// Should receive initial ADDED events and a BOOKMARK
	addedCount := 0
	bookmarkReceived := false

	for i := 0; i < 3; i++ {
		select {
		case event := <-w.ResultChan():
			switch event.Type {
			case watch.Added:
				addedCount++
			case watch.Bookmark:
				bookmarkReceived = true
			}
		case <-time.After(time.Second):
			t.Fatalf("Timeout waiting for event %d", i)
		}
	}

	if addedCount != 2 {
		t.Errorf("Expected 2 Added events, got %d", addedCount)
	}
	if !bookmarkReceived {
		t.Error("Expected bookmark event")
	}
}

func TestNodeMetricsWatchHelper_WatchWithInitialEvents(t *testing.T) {
	node1 := metrics.NodeMetrics{}
	node1.Name = "node1"

	node2 := metrics.NodeMetrics{}
	node2.Name = "node2"

	storage := &fakeWatchableNodeStorage{
		currentRV:  "42",
		allMetrics: []metrics.NodeMetrics{node1, node2},
	}

	helper := NewNodeMetricsWatchHelper(storage, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := helper.Watch(ctx, labels.Everything(), true)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}
	defer w.Stop()

	// Should receive initial ADDED events and a BOOKMARK
	addedCount := 0
	bookmarkReceived := false

	for i := 0; i < 3; i++ {
		select {
		case event := <-w.ResultChan():
			switch event.Type {
			case watch.Added:
				addedCount++
			case watch.Bookmark:
				bookmarkReceived = true
			}
		case <-time.After(time.Second):
			t.Fatalf("Timeout waiting for event %d", i)
		}
	}

	if addedCount != 2 {
		t.Errorf("Expected 2 Added events, got %d", addedCount)
	}
	if !bookmarkReceived {
		t.Error("Expected bookmark event")
	}
}

func TestMetricsWatcher_BookmarkType(t *testing.T) {
	// Test that pod watchers get PodMetrics bookmarks
	t.Run("pod watcher gets PodMetrics bookmark", func(t *testing.T) {
		w := newMetricsWatcher("", labels.Everything(), resourceTypePod, nil)
		defer w.Stop()

		// Send bookmark directly - it goes to result channel
		ok := w.sendBookmark("123")
		if !ok {
			t.Fatal("sendBookmark returned false")
		}

		select {
		case event := <-w.ResultChan():
			if event.Type != watch.Bookmark {
				t.Errorf("Expected Bookmark, got %v", event.Type)
			}
			_, ok := event.Object.(*metrics.PodMetrics)
			if !ok {
				t.Errorf("Expected *metrics.PodMetrics, got %T", event.Object)
			}
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for bookmark")
		}
	})

	// Test that node watchers get NodeMetrics bookmarks
	t.Run("node watcher gets NodeMetrics bookmark", func(t *testing.T) {
		w := newMetricsWatcher("", labels.Everything(), resourceTypeNode, nil)
		defer w.Stop()

		ok := w.sendBookmark("456")
		if !ok {
			t.Fatal("sendBookmark returned false")
		}

		select {
		case event := <-w.ResultChan():
			if event.Type != watch.Bookmark {
				t.Errorf("Expected Bookmark, got %v", event.Type)
			}
			_, ok := event.Object.(*metrics.NodeMetrics)
			if !ok {
				t.Errorf("Expected *metrics.NodeMetrics, got %T", event.Object)
			}
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for bookmark")
		}
	})
}

func TestMetricsWatcher_LabelEnrichment(t *testing.T) {
	// Simulate: storage events have no labels, but labelLookup provides them
	lookup := func(namespace, name string) map[string]string {
		if name == "pod1" {
			return map[string]string{"app": "myapp"}
		}
		if name == "pod2" {
			return map[string]string{"app": "other"}
		}
		return nil
	}

	selector, _ := labels.Parse("app=myapp")
	w := newMetricsWatcher("", selector, resourceTypePod, lookup)
	defer w.Stop()

	// Send events WITHOUT labels (as storage does)
	pod1 := metrics.PodMetrics{}
	pod1.Name = "pod1"
	pod1.Namespace = "default"
	// Note: no Labels set

	pod2 := metrics.PodMetrics{}
	pod2.Name = "pod2"
	pod2.Namespace = "default"

	w.Send(WatchEvent{Type: watch.Modified, Object: pod1})
	w.Send(WatchEvent{Type: watch.Modified, Object: pod2})

	// Should receive pod1 (labels enriched from lookup → matches selector)
	select {
	case event := <-w.ResultChan():
		pm := event.Object.(*metrics.PodMetrics)
		if pm.Name != "pod1" {
			t.Errorf("Expected pod1, got %s", pm.Name)
		}
		if pm.Labels["app"] != "myapp" {
			t.Errorf("Expected enriched labels, got %v", pm.Labels)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for enriched event")
	}

	// pod2 should be filtered out (labels enriched → doesn't match selector)
	select {
	case event := <-w.ResultChan():
		t.Errorf("Unexpected event: %v", event)
	case <-time.After(100 * time.Millisecond):
		// Expected
	}
}

// fakeWatchablePodStorage implements WatchablePodMetricsGetter for testing
type fakeWatchablePodStorage struct {
	currentRV  string
	allMetrics []metrics.PodMetrics
	watchers   map[uint64]MetricsWatcher
	nextID     uint64
}

func (f *fakeWatchablePodStorage) GetPodMetrics(pods ...*metav1.PartialObjectMetadata) ([]metrics.PodMetrics, error) {
	return f.allMetrics, nil
}

func (f *fakeWatchablePodStorage) CurrentResourceVersion() string {
	return f.currentRV
}

func (f *fakeWatchablePodStorage) GetAllPodMetrics() []metrics.PodMetrics {
	return f.allMetrics
}

func (f *fakeWatchablePodStorage) RegisterPodWatcher(w MetricsWatcher) uint64 {
	if f.watchers == nil {
		f.watchers = make(map[uint64]MetricsWatcher)
	}
	f.nextID++
	f.watchers[f.nextID] = w
	return f.nextID
}

func (f *fakeWatchablePodStorage) UnregisterPodWatcher(id uint64) {
	delete(f.watchers, id)
}

func (f *fakeWatchablePodStorage) RegisterPodWatcherWithSnapshot(w MetricsWatcher) (id uint64, allMetrics []metrics.PodMetrics, rv string) {
	if f.watchers == nil {
		f.watchers = make(map[uint64]MetricsWatcher)
	}
	f.nextID++
	f.watchers[f.nextID] = w
	return f.nextID, f.allMetrics, f.currentRV
}

// fakeWatchableNodeStorage implements WatchableNodeMetricsGetter for testing
type fakeWatchableNodeStorage struct {
	currentRV  string
	allMetrics []metrics.NodeMetrics
	watchers   map[uint64]MetricsWatcher
	nextID     uint64
}

func (f *fakeWatchableNodeStorage) GetNodeMetrics(nodes ...*corev1.Node) ([]metrics.NodeMetrics, error) {
	return f.allMetrics, nil
}

func (f *fakeWatchableNodeStorage) CurrentResourceVersion() string {
	return f.currentRV
}

func (f *fakeWatchableNodeStorage) GetAllNodeMetrics() []metrics.NodeMetrics {
	return f.allMetrics
}

func (f *fakeWatchableNodeStorage) RegisterNodeWatcher(w MetricsWatcher) uint64 {
	if f.watchers == nil {
		f.watchers = make(map[uint64]MetricsWatcher)
	}
	f.nextID++
	f.watchers[f.nextID] = w
	return f.nextID
}

func (f *fakeWatchableNodeStorage) UnregisterNodeWatcher(id uint64) {
	delete(f.watchers, id)
}

func (f *fakeWatchableNodeStorage) RegisterNodeWatcherWithSnapshot(w MetricsWatcher) (id uint64, allMetrics []metrics.NodeMetrics, rv string) {
	if f.watchers == nil {
		f.watchers = make(map[uint64]MetricsWatcher)
	}
	f.nextID++
	f.watchers[f.nextID] = w
	return f.nextID, f.allMetrics, f.currentRV
}
