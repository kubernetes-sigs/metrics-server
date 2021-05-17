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
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/metrics/pkg/apis/metrics"

	"sigs.k8s.io/metrics-server/pkg/api"
)

// nodeStorage is a thread save nodeStorage for node and pod metrics.
type storage struct {
	mu    sync.RWMutex
	pods  podStorage
	nodes nodeStorage
}

var _ Storage = (*storage)(nil)

func NewStorage(metricResolution time.Duration) *storage {
	return &storage{pods: podStorage{metricResolution: metricResolution}}
}

// Ready returns true if metrics-server's storage has accumulated enough metric
// points to serve NodeMetrics.
func (s *storage) Ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.nodes.prev) != 0 || len(s.pods.prev) != 0
}

func (s *storage) GetNodeMetrics(nodes ...string) ([]api.TimeInfo, []corev1.ResourceList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nodes.GetMetrics(nodes...)
}

func (s *storage) GetPodMetrics(pods ...apitypes.NamespacedName) ([]api.TimeInfo, [][]metrics.ContainerMetrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pods.GetMetrics(pods...)
}

func (s *storage) Store(batch *MetricsBatch) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes.Store(batch.Nodes)
	s.pods.Store(batch.Pods)
}
