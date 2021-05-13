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
	"k8s.io/apimachinery/pkg/api/resource"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/metrics"

	"sigs.k8s.io/metrics-server/pkg/api"
)

// storage is a thread save storage for node and pod metrics.
//
// This implementation only stores metric points if they are newer than the
// points already stored and the cpuUsageOverTime function used to handle
// cumulative metrics assumes that the time window is different from 0.
type storage struct {
	mu sync.RWMutex
	// lastNodes stores node metric points from last scrape
	lastNodes map[string]NodeMetricsPoint
	// prevNodes stores node metric points from scrape preceding the last one.
	// Points timestamp should proceed the corresponding points from lastNodes and have same start time (no restart between them).
	prevNodes map[string]NodeMetricsPoint
	// lastPods stores pod metric points from last scrape
	lastPods map[apitypes.NamespacedName]map[string]ContainerMetricsPoint
	// prevPods stores pod metric points from scrape preceding the last one.
	// Points timestamp should proceed the corresponding points from lastPods and have same start time (no restart between them).
	prevPods map[apitypes.NamespacedName]map[string]ContainerMetricsPoint
}

var _ Storage = (*storage)(nil)

func NewStorage() *storage {
	return &storage{}
}

// Ready returns true if metrics-server's storage has accumulated enough metric
// points to serve NodeMetrics.
// Checking only nodes for metrics-server's storage readiness is enough to make
// sure that it has accumulated enough metrics to serve both NodeMetrics and
// PodMetrics. It also covers cases where metrics-server only has to serve
// NodeMetrics.
func (p *storage) Ready() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.prevNodes) != 0
}

func (p *storage) GetNodeMetrics(nodes ...string) ([]api.TimeInfo, []corev1.ResourceList, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ts := make([]api.TimeInfo, len(nodes))
	ms := make([]corev1.ResourceList, len(nodes))
	for i, node := range nodes {
		last, found := p.lastNodes[node]
		if !found {
			continue
		}

		prev, found := p.prevNodes[node]
		if !found {
			continue
		}

		cpuUsage := cpuUsageOverTime(last.MetricsPoint, prev.MetricsPoint)
		ms[i] = corev1.ResourceList{
			corev1.ResourceCPU:    cpuUsage,
			corev1.ResourceMemory: last.MemoryUsage,
		}
		ts[i] = api.TimeInfo{
			Timestamp: last.Timestamp,
			Window:    last.Timestamp.Sub(prev.Timestamp),
		}
	}
	return ts, ms, nil
}

func (p *storage) GetContainerMetrics(pods ...apitypes.NamespacedName) ([]api.TimeInfo, [][]metrics.ContainerMetrics, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ts := make([]api.TimeInfo, len(pods))
	ms := make([][]metrics.ContainerMetrics, len(pods))
	for i, pod := range pods {
		lastPod, found := p.lastPods[pod]
		if !found {
			continue
		}

		prevPod, found := p.prevPods[pod]
		if !found {
			continue
		}

		var (
			cms        = make([]metrics.ContainerMetrics, 0, len(lastPod))
			earliestTS time.Time
			window     time.Duration
		)
		for container, lastContainer := range lastPod {
			prevContainer, found := prevPod[container]
			if !found {
				continue
			}

			cpuUsage := cpuUsageOverTime(lastContainer.MetricsPoint, prevContainer.MetricsPoint)
			cms = append(cms, metrics.ContainerMetrics{
				Name: lastContainer.Name,
				Usage: corev1.ResourceList{
					corev1.ResourceCPU:    cpuUsage,
					corev1.ResourceMemory: lastContainer.MemoryUsage,
				},
			})
			if earliestTS.IsZero() || earliestTS.After(lastContainer.Timestamp) {
				earliestTS = lastContainer.Timestamp
				window = lastContainer.Timestamp.Sub(prevContainer.Timestamp)
			}
		}
		ms[i] = cms
		ts[i] = api.TimeInfo{
			Timestamp: earliestTS,
			Window:    window,
		}
	}
	return ts, ms, nil
}

func (p *storage) Store(batch *MetricsBatch) {
	p.storeNodeMetrics(batch.Nodes)
	p.storePodMetrics(batch.Pods)
}

func (p *storage) storeNodeMetrics(newNodes []NodeMetricsPoint) {
	p.mu.Lock()
	defer p.mu.Unlock()

	lastNodes := make(map[string]NodeMetricsPoint, len(newNodes))
	prevNodes := make(map[string]NodeMetricsPoint, len(newNodes))
	for _, newNode := range newNodes {
		if _, exists := lastNodes[newNode.Name]; exists {
			klog.ErrorS(nil, "Got duplicate node point", "node", klog.KRef("", newNode.Name))
			continue
		}
		lastNodes[newNode.Name] = newNode

		// Keep previous metric point if newNode has not restarted (new metric start time < stored timestamp)
		if lastNode, found := p.lastNodes[newNode.Name]; found && newNode.StartTime.Before(lastNode.Timestamp) {
			// If new point is different then one already stored
			if newNode.Timestamp.After(lastNode.Timestamp) {
				// Move stored point to previous
				prevNodes[newNode.Name] = lastNode
			} else if prevNode, found := p.prevNodes[newNode.Name]; found {
				// Keep previous point
				prevNodes[newNode.Name] = prevNode
			}
		}
	}
	p.lastNodes = lastNodes
	p.prevNodes = prevNodes

	// Only count lastNodes for which metrics can be returned.
	pointsStored.WithLabelValues("node").Set(float64(len(prevNodes)))
}

func (p *storage) storePodMetrics(newPods []PodMetricsPoint) {
	p.mu.Lock()
	defer p.mu.Unlock()

	lastPods := make(map[apitypes.NamespacedName]map[string]ContainerMetricsPoint, len(newPods))
	prevPods := make(map[apitypes.NamespacedName]map[string]ContainerMetricsPoint, len(newPods))
	var containerCount int
	for _, newPod := range newPods {
		podRef := apitypes.NamespacedName{Name: newPod.Name, Namespace: newPod.Namespace}
		if _, found := lastPods[podRef]; found {
			klog.ErrorS(nil, "Got duplicate pod point", "pod", klog.KRef(newPod.Namespace, newPod.Name))
			continue
		}

		lastContainers := make(map[string]ContainerMetricsPoint, len(newPod.Containers))
		prevContainers := make(map[string]ContainerMetricsPoint, len(newPod.Containers))
		for _, newContainer := range newPod.Containers {
			if _, exists := lastContainers[newContainer.Name]; exists {
				klog.ErrorS(nil, "Got duplicate Container point", "container", newContainer.Name, "pod", klog.KRef(newPod.Namespace, newPod.Name))
				continue
			}
			lastContainers[newContainer.Name] = newContainer

			if lastPod, found := p.lastPods[podRef]; found {
				// Keep previous metric point if newContainer has not restarted (new metric start time < stored timestamp)
				if lastContainer, found := lastPod[newContainer.Name]; found && newContainer.StartTime.Before(lastContainer.Timestamp) {
					// If new point is different then one already stored
					if newContainer.Timestamp.After(lastContainer.Timestamp) {
						// Move stored point to previous
						prevContainers[newContainer.Name] = lastContainer
					} else if prevPod, found := p.prevPods[podRef]; found {
						// Keep previous point
						prevContainers[newContainer.Name] = prevPod[newContainer.Name]
					}
				}
			}
		}
		containerPoints := len(prevContainers)
		if containerPoints > 0 {
			prevPods[podRef] = prevContainers
		}
		lastPods[podRef] = lastContainers

		// Only count containers for which metrics can be returned.
		containerCount += containerPoints
	}
	p.lastPods = lastPods
	p.prevPods = prevPods

	pointsStored.WithLabelValues("container").Set(float64(containerCount))
}

func cpuUsageOverTime(last, prev MetricsPoint) resource.Quantity {
	window := last.Timestamp.Sub(prev.Timestamp).Seconds()
	lastUsage := last.CumulativeCpuUsed.ScaledValue(-9)
	prevUsage := prev.CumulativeCpuUsed.ScaledValue(-9)
	cpuUsage := float64(lastUsage-prevUsage) / window
	return *resource.NewScaledQuantity(int64(cpuUsage), -9)
}
