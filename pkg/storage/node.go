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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/metrics"
)

// nodeStorage stores last two node metric batches and calculates cpu & memory usage
//
// This implementation only stores metric points if they are newer than the
// points already stored and the cpuUsageOverTime function used to handle
// cumulative metrics assumes that the time window is different from 0.
type nodeStorage struct {
	// last stores node metric points from last scrape
	last map[string]MetricsPoint
	// prev stores node metric points from scrape preceding the last one.
	// Points timestamp should proceed the corresponding points from last.
	prev map[string]MetricsPoint
}

func (s *nodeStorage) GetMetrics(nodes ...*corev1.Node) ([]metrics.NodeMetrics, error) {
	results := make([]metrics.NodeMetrics, 0, len(nodes))
	for _, node := range nodes {
		last, found := s.last[node.Name]
		if !found {
			continue
		}

		prev, found := s.prev[node.Name]
		if !found {
			continue
		}
		rl, ti, err := resourceUsage(last, prev)
		if err != nil {
			klog.ErrorS(err, "Skipping node usage metric", "node", node)
			continue
		}
		results = append(results, metrics.NodeMetrics{
			ObjectMeta: metav1.ObjectMeta{
				Name:              node.Name,
				Labels:            node.Labels,
				CreationTimestamp: metav1.NewTime(time.Now()),
			},
			Timestamp: metav1.NewTime(ti.Timestamp),
			Window:    metav1.Duration{Duration: ti.Window},
			Usage:     rl,
		})
	}
	return results, nil
}

func (s *nodeStorage) Store(batch *MetricsBatch) {
	lastNodes := make(map[string]MetricsPoint, len(batch.Nodes))
	prevNodes := make(map[string]MetricsPoint, len(batch.Nodes))
	for nodeName, newPoint := range batch.Nodes {
		if _, exists := lastNodes[nodeName]; exists {
			klog.ErrorS(nil, "Got duplicate node point", "node", klog.KRef("", nodeName))
			continue
		}
		lastNodes[nodeName] = newPoint

		if lastNode, found := s.last[nodeName]; found {
			// If new point is different then one already stored
			if newPoint.Timestamp.After(lastNode.Timestamp) {
				// Move stored point to previous
				prevNodes[nodeName] = lastNode
			} else if prevPoint, found := s.prev[nodeName]; found {
				if prevPoint.Timestamp.Before(newPoint.Timestamp) {
					// Keep previous point
					prevNodes[nodeName] = prevPoint
				} else {
					klog.V(2).InfoS("Found new node metrics point is older than stored previous, drop previous",
						"node", nodeName,
						"previousTimestamp", prevPoint.Timestamp,
						"timestamp", newPoint.Timestamp)
				}
			}
		}
	}
	s.last = lastNodes
	s.prev = prevNodes

	// Only count last for which metrics can be returned.
	pointsStored.WithLabelValues("node").Set(float64(len(prevNodes)))
}
