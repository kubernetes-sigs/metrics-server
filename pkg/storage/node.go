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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"sigs.k8s.io/metrics-server/pkg/api"
)

// nodeStorage stores last two node metric batches and calculates cpu & memory usage
//
// This implementation only stores metric points if they are newer than the
// points already stored and the cpuUsageOverTime function used to handle
// cumulative metrics assumes that the time window is different from 0.
type nodeStorage struct {
	// last stores node metric points from last scrape
	last map[string]NodeMetricsPoint
	// prev stores node metric points from scrape preceding the last one.
	// Points timestamp should proceed the corresponding points from last and have same start time (no restart between them).
	prev map[string]NodeMetricsPoint
}

func (s *nodeStorage) GetMetrics(nodes ...string) ([]api.TimeInfo, []corev1.ResourceList, error) {
	tis := make([]api.TimeInfo, len(nodes))
	rls := make([]corev1.ResourceList, len(nodes))
	for i, node := range nodes {
		last, found := s.last[node]
		if !found {
			continue
		}

		prev, found := s.prev[node]
		if !found {
			continue
		}
		rl, ti, err := resourceUsage(last.MetricsPoint, prev.MetricsPoint)
		if err != nil {
			klog.ErrorS(err, "Skipping node usage metric", "node", node)
			continue
		}
		rls[i] = rl
		tis[i] = ti
	}
	return tis, rls, nil
}

func (s *nodeStorage) Store(newNodes []NodeMetricsPoint) {
	lastNodes := make(map[string]NodeMetricsPoint, len(newNodes))
	prevNodes := make(map[string]NodeMetricsPoint, len(newNodes))
	for _, newNode := range newNodes {
		if _, exists := lastNodes[newNode.Name]; exists {
			klog.ErrorS(nil, "Got duplicate node point", "node", klog.KRef("", newNode.Name))
			continue
		}
		lastNodes[newNode.Name] = newNode

		// Keep previous metric point if newNode has not restarted (new metric start time < stored timestamp)
		if lastNode, found := s.last[newNode.Name]; found && newNode.StartTime.Before(lastNode.Timestamp) {
			// If new point is different then one already stored
			if newNode.Timestamp.After(lastNode.Timestamp) {
				// Move stored point to previous
				prevNodes[newNode.Name] = lastNode
			} else if prevNode, found := s.prev[newNode.Name]; found {
				if prevNode.Timestamp.Before(newNode.Timestamp) {
					// Keep previous point
					prevNodes[newNode.Name] = prevNode
				} else {
					klog.V(2).InfoS("Found new node metrics point is older then stored previous, drop previous",
						"node", newNode.Name,
						"previousTimestamp", prevNode.Timestamp,
						"timestamp", newNode.Timestamp)
				}
			}
		}
	}
	s.last = lastNodes
	s.prev = prevNodes

	// Only count last for which metrics can be returned.
	pointsStored.WithLabelValues("node").Set(float64(len(prevNodes)))
}
