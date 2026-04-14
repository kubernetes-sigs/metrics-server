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
	"sync"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/metrics/pkg/apis/metrics"
)

const (
	// watcherBufferSize is the number of events that can be buffered per watcher
	// before the watcher is closed (slow consumer protection)
	watcherBufferSize = 1000

	// Resource types for bookmark generation
	resourceTypePod  = "pod"
	resourceTypeNode = "node"
)

// LabelLookupFunc returns labels for a given resource identified by namespace and name.
// For cluster-scoped resources (nodes), namespace is empty.
type LabelLookupFunc func(namespace, name string) map[string]string

// metricsWatcher implements watch.Interface for metrics resources.
// It receives events from the storage layer and filters them based on
// namespace and label selector before forwarding to clients.
type metricsWatcher struct {
	result    chan watch.Event
	done      chan struct{}
	closeOnce sync.Once

	// Filter criteria
	namespace     string          // empty means all namespaces
	labelSelector labels.Selector

	// Resource type for correct bookmark generation
	resourceType string // "pod" or "node"

	// labelLookup enriches events with labels from the authoritative source (pod/node lister).
	// Storage-generated events don't carry labels, so this is needed for label selector filtering.
	labelLookup LabelLookupFunc
}

var _ watch.Interface = &metricsWatcher{}
var _ MetricsWatcher = &metricsWatcher{}

// newMetricsWatcher creates a new watcher with the given filter criteria.
func newMetricsWatcher(namespace string, labelSelector labels.Selector, resourceType string, labelLookup LabelLookupFunc) *metricsWatcher {
	if labelSelector == nil {
		labelSelector = labels.Everything()
	}
	return &metricsWatcher{
		result:        make(chan watch.Event, watcherBufferSize),
		done:          make(chan struct{}),
		namespace:     namespace,
		labelSelector: labelSelector,
		resourceType:  resourceType,
		labelLookup:   labelLookup,
	}
}

// Stop implements watch.Interface
func (w *metricsWatcher) Stop() {
	w.closeOnce.Do(func() {
		close(w.done)
		// Note: We don't close w.result here to avoid panics from concurrent sends.
		// Instead, readers should use the select pattern:
		//   select {
		//   case event, ok := <-w.ResultChan():
		//       if !ok { return } // channel closed
		//   case <-w.Done():
		//       return // watcher stopped
		//   }
		// Since we close w.done, all blocking sends will unblock and return false.
		// Readers that are blocked on ResultChan will unblock when they check Done.
	})
}

// ResultChan implements watch.Interface
func (w *metricsWatcher) ResultChan() <-chan watch.Event {
	return w.result
}

// Done implements MetricsWatcher
func (w *metricsWatcher) Done() <-chan struct{} {
	return w.done
}

// Send implements MetricsWatcher. Returns false if the watcher is closed or full.
func (w *metricsWatcher) Send(event WatchEvent) bool {
	// Check if watcher is stopped
	select {
	case <-w.done:
		return false
	default:
	}

	// Enrich object with labels from the authoritative source if available
	obj := w.enrichLabels(event.Object)

	// Filter the event
	if !w.matchesFilter(obj) {
		return true // Event filtered out, but watcher is still alive
	}

	// Convert to watch.Event
	watchEvent := watch.Event{
		Type:   event.Type,
		Object: w.toRuntimeObject(obj),
	}

	// Try to send, close if buffer is full (slow consumer)
	select {
	case w.result <- watchEvent:
		return true
	case <-w.done:
		return false
	default:
		// Buffer full, close the watcher
		w.Stop()
		return false
	}
}

// matchesFilter checks if the object matches the watcher's filter criteria
func (w *metricsWatcher) matchesFilter(obj interface{}) bool {
	switch m := obj.(type) {
	case metrics.PodMetrics:
		// Check namespace filter
		if w.namespace != "" && m.Namespace != w.namespace {
			return false
		}
		// Check label selector
		return w.labelSelector.Matches(labels.Set(m.Labels))
	case metrics.NodeMetrics:
		// Nodes are cluster-scoped, no namespace filter
		return w.labelSelector.Matches(labels.Set(m.Labels))
	default:
		return false
	}
}

// enrichLabels looks up current labels for the given metrics object.
// Storage-generated events don't carry labels, so this uses the label lookup
// function (backed by pod/node listers) to add them.
func (w *metricsWatcher) enrichLabels(obj interface{}) interface{} {
	if w.labelLookup == nil {
		return obj
	}
	switch m := obj.(type) {
	case metrics.PodMetrics:
		if m.Labels == nil {
			m.Labels = w.labelLookup(m.Namespace, m.Name)
		}
		return m
	case metrics.NodeMetrics:
		if m.Labels == nil {
			m.Labels = w.labelLookup("", m.Name)
		}
		return m
	default:
		return obj
	}
}

// toRuntimeObject converts the metrics object to a runtime.Object
func (w *metricsWatcher) toRuntimeObject(obj interface{}) runtime.Object {
	switch m := obj.(type) {
	case metrics.PodMetrics:
		return &m
	case metrics.NodeMetrics:
		return &m
	default:
		return nil
	}
}

// sendInitialEvents sends the initial ADDED events and a BOOKMARK for WatchList semantics.
// This should be called after creating the watcher but before registering it with the store.
func (w *metricsWatcher) sendInitialEvents(objects []runtime.Object, resourceVersion string) bool {
	for _, obj := range objects {
		if !w.matchesFilterRuntime(obj) {
			continue
		}

		event := watch.Event{
			Type:   watch.Added,
			Object: obj,
		}

		select {
		case w.result <- event:
		case <-w.done:
			return false
		default:
			// Buffer full
			w.Stop()
			return false
		}
	}

	// Send bookmark event
	return w.sendBookmark(resourceVersion)
}

// matchesFilterRuntime checks filter criteria for runtime.Object
func (w *metricsWatcher) matchesFilterRuntime(obj runtime.Object) bool {
	switch m := obj.(type) {
	case *metrics.PodMetrics:
		if w.namespace != "" && m.Namespace != w.namespace {
			return false
		}
		return w.labelSelector.Matches(labels.Set(m.Labels))
	case *metrics.NodeMetrics:
		return w.labelSelector.Matches(labels.Set(m.Labels))
	default:
		return false
	}
}

// sendBookmark sends a BOOKMARK event with the given resource version
func (w *metricsWatcher) sendBookmark(resourceVersion string) bool {
	var bookmark runtime.Object
	switch w.resourceType {
	case resourceTypeNode:
		obj := &metrics.NodeMetrics{}
		obj.ResourceVersion = resourceVersion
		bookmark = obj
	default: // resourceTypePod
		obj := &metrics.PodMetrics{}
		obj.ResourceVersion = resourceVersion
		bookmark = obj
	}

	event := watch.Event{
		Type:   watch.Bookmark,
		Object: bookmark,
	}

	select {
	case w.result <- event:
		return true
	case <-w.done:
		return false
	default:
		w.Stop()
		return false
	}
}

// PodMetricsWatchHelper helps create watches for pod metrics
type PodMetricsWatchHelper struct {
	storage     WatchablePodMetricsGetter
	labelLookup LabelLookupFunc
}

// NewPodMetricsWatchHelper creates a new watch helper for pod metrics.
// labelLookup provides labels for pod metrics objects (used for label selector filtering).
func NewPodMetricsWatchHelper(storage WatchablePodMetricsGetter, labelLookup LabelLookupFunc) *PodMetricsWatchHelper {
	return &PodMetricsWatchHelper{storage: storage, labelLookup: labelLookup}
}

// Watch creates a new watch for pod metrics with the given filters.
// If sendInitialEvents is true, it sends all current metrics as ADDED events
// followed by a BOOKMARK before streaming updates.
func (h *PodMetricsWatchHelper) Watch(ctx context.Context, namespace string, labelSelector labels.Selector, sendInitialEvents bool) (watch.Interface, error) {
	w := newMetricsWatcher(namespace, labelSelector, resourceTypePod, h.labelLookup)

	var watcherID uint64

	if sendInitialEvents {
		// Use atomic registration to prevent race condition where Store()
		// could fire between getting the snapshot and registering the watcher
		var allMetrics []metrics.PodMetrics
		var rv string
		watcherID, allMetrics, rv = h.storage.RegisterPodWatcherWithSnapshot(w)

		// Convert to runtime.Object slice
		objects := make([]runtime.Object, len(allMetrics))
		for i := range allMetrics {
			objects[i] = &allMetrics[i]
		}

		// Send initial events
		if !w.sendInitialEvents(objects, rv) {
			h.storage.UnregisterPodWatcher(watcherID)
			return w, nil
		}
	} else {
		// No initial events needed, just register
		watcherID = h.storage.RegisterPodWatcher(w)
	}

	// Set up cleanup on context cancellation
	go func() {
		select {
		case <-ctx.Done():
			w.Stop()
			h.storage.UnregisterPodWatcher(watcherID)
		case <-w.done:
			h.storage.UnregisterPodWatcher(watcherID)
		}
	}()

	return w, nil
}

// NodeMetricsWatchHelper helps create watches for node metrics
type NodeMetricsWatchHelper struct {
	storage     WatchableNodeMetricsGetter
	labelLookup LabelLookupFunc
}

// NewNodeMetricsWatchHelper creates a new watch helper for node metrics.
// labelLookup provides labels for node metrics objects (used for label selector filtering).
func NewNodeMetricsWatchHelper(storage WatchableNodeMetricsGetter, labelLookup LabelLookupFunc) *NodeMetricsWatchHelper {
	return &NodeMetricsWatchHelper{storage: storage, labelLookup: labelLookup}
}

// Watch creates a new watch for node metrics with the given filters.
// If sendInitialEvents is true, it sends all current metrics as ADDED events
// followed by a BOOKMARK before streaming updates.
func (h *NodeMetricsWatchHelper) Watch(ctx context.Context, labelSelector labels.Selector, sendInitialEvents bool) (watch.Interface, error) {
	w := newMetricsWatcher("", labelSelector, resourceTypeNode, h.labelLookup) // Nodes are cluster-scoped

	var watcherID uint64

	if sendInitialEvents {
		// Use atomic registration to prevent race condition where Store()
		// could fire between getting the snapshot and registering the watcher
		var allMetrics []metrics.NodeMetrics
		var rv string
		watcherID, allMetrics, rv = h.storage.RegisterNodeWatcherWithSnapshot(w)

		// Convert to runtime.Object slice
		objects := make([]runtime.Object, len(allMetrics))
		for i := range allMetrics {
			objects[i] = &allMetrics[i]
		}

		// Send initial events
		if !w.sendInitialEvents(objects, rv) {
			h.storage.UnregisterNodeWatcher(watcherID)
			return w, nil
		}
	} else {
		// No initial events needed, just register
		watcherID = h.storage.RegisterNodeWatcher(w)
	}

	// Set up cleanup on context cancellation
	go func() {
		select {
		case <-ctx.Done():
			w.Stop()
			h.storage.UnregisterNodeWatcher(watcherID)
		case <-w.done:
			h.storage.UnregisterNodeWatcher(watcherID)
		}
	}()

	return w, nil
}
