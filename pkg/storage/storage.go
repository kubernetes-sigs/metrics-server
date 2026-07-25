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
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/metrics"
)

// nodeStorage is a thread save nodeStorage for node and pod metrics.
type storage struct {
	mu    sync.RWMutex
	pods  podStorage
	nodes nodeStorage

	prevNodesReady bool
	prevPodsReady  bool
}

var _ Storage = (*storage)(nil)

func NewStorage(metricResolution time.Duration) *storage {
	return &storage{pods: podStorage{metricResolution: metricResolution}}
}

// Ready returns true if metrics-server's storage has accumulated enough metric
// points to serve both NodeMetrics and PodMetrics.
func (s *storage) Ready() bool {
	return s.NodeReady() && s.PodReady()
}

// NodeReady returns true if metrics-server's storage has accumulated enough
// metric points to serve NodeMetrics.
func (s *storage) NodeReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nodeReady()
}

// PodReady returns true if metrics-server's storage has accumulated enough
// metric points to serve PodMetrics.
func (s *storage) PodReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.podReady()
}

func (s *storage) nodeReady() bool {
	return len(s.nodes.prev) != 0
}

func (s *storage) podReady() bool {
	return len(s.pods.prev) != 0
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

func (s *storage) Store(batch *MetricsBatch) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes.Store(batch)
	s.pods.Store(batch)
	s.logReadinessChange()
}

func (s *storage) logReadinessChange() {
	nodesReady := s.nodeReady()
	podsReady := s.podReady()

	if nodesReady == s.prevNodesReady && podsReady == s.prevPodsReady {
		return
	}

	switch {
	case nodesReady && podsReady:
		klog.V(2).InfoS("Metric storage is ready", "nodeMetrics", nodesReady, "podMetrics", podsReady)
	case nodesReady && !podsReady:
		klog.V(2).InfoS("Metric storage is not ready, pod metrics are missing, this may indicate a container-runtime or kubelet issue", "nodeMetrics", nodesReady, "podMetrics", podsReady)
	case !nodesReady && podsReady:
		klog.V(2).InfoS("Metric storage is not ready, node metrics are missing, this may indicate a kubelet issue", "nodeMetrics", nodesReady, "podMetrics", podsReady)
	default:
		klog.V(2).InfoS("Metric storage is not ready", "nodeMetrics", nodesReady, "podMetrics", podsReady)
	}

	s.prevNodesReady = nodesReady
	s.prevPodsReady = podsReady
}
