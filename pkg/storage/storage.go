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
	mu        sync.RWMutex
	nodes     map[string]NodeMetricsPoint
	prevNodes map[string]NodeMetricsPoint
	pods      map[apitypes.NamespacedName]map[string]ContainerMetricsPoint
	prevPods  map[apitypes.NamespacedName]map[string]ContainerMetricsPoint
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

	timestamps := make([]api.TimeInfo, len(nodes))
	resMetrics := make([]corev1.ResourceList, len(nodes))
	for i, node := range nodes {
		metricPoint, found := p.nodes[node]
		if !found {
			continue
		}

		prevMetricPoint, found := p.prevNodes[node]
		if !found {
			continue
		}

		cpuUsage := cpuUsageOverTime(metricPoint.MetricsPoint, prevMetricPoint.MetricsPoint)
		resMetrics[i] = corev1.ResourceList{
			corev1.ResourceName(corev1.ResourceCPU):    cpuUsage,
			corev1.ResourceName(corev1.ResourceMemory): metricPoint.MemoryUsage,
		}
		timestamps[i] = api.TimeInfo{
			Timestamp: metricPoint.Timestamp,
			Window:    metricPoint.Timestamp.Sub(prevMetricPoint.Timestamp),
		}
	}
	return timestamps, resMetrics, nil
}

func (p *storage) GetContainerMetrics(pods ...apitypes.NamespacedName) ([]api.TimeInfo, [][]metrics.ContainerMetrics, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	timestamps := make([]api.TimeInfo, len(pods))
	resMetrics := make([][]metrics.ContainerMetrics, len(pods))
	for podIdx, pod := range pods {
		contPoints, found := p.pods[pod]
		if !found {
			continue
		}

		prevPod, found := p.prevPods[pod]
		if !found {
			continue
		}

		var (
			contMetrics = make([]metrics.ContainerMetrics, 0, len(contPoints))
			earliestTS  time.Time
			window      time.Duration
		)
		for contName, contPoint := range contPoints {
			prevContPoint, found := prevPod[contName]
			if !found {
				continue
			}

			cpuUsage := cpuUsageOverTime(contPoint.MetricsPoint, prevContPoint.MetricsPoint)
			contMetrics = append(contMetrics, metrics.ContainerMetrics{
				Name: contPoint.Name,
				Usage: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    cpuUsage,
					corev1.ResourceName(corev1.ResourceMemory): contPoint.MemoryUsage,
				},
			})
			if earliestTS.IsZero() || earliestTS.After(contPoint.Timestamp) {
				earliestTS = contPoint.Timestamp
				window = contPoint.Timestamp.Sub(prevContPoint.Timestamp)
			}
		}
		resMetrics[podIdx] = contMetrics
		timestamps[podIdx] = api.TimeInfo{
			Timestamp: earliestTS,
			Window:    window,
		}
	}
	return timestamps, resMetrics, nil
}

func (p *storage) Store(batch *MetricsBatch) {
	p.storeNodeMetrics(batch.Nodes)
	p.storePodMetrics(batch.Pods)
}

func (p *storage) storeNodeMetrics(nodes []NodeMetricsPoint) {
	p.mu.Lock()
	defer p.mu.Unlock()

	newNodes := make(map[string]NodeMetricsPoint, len(nodes))
	prevNodes := make(map[string]NodeMetricsPoint, len(nodes))
	for _, nodePoint := range nodes {
		if _, exists := newNodes[nodePoint.Name]; exists {
			klog.ErrorS(nil, "Got duplicate node point", "node", klog.KRef("", nodePoint.Name))
			continue
		}
		newNodes[nodePoint.Name] = nodePoint

		// If the new point is newer than the one stored for the container, move
		// it to the list of the previous points.
		// This check also prevents from updating the store if the same metric
		// point was scraped twice.
		storedNodePoint, found := p.nodes[nodePoint.Name]
		if found && nodePoint.Timestamp.After(storedNodePoint.Timestamp) {
			prevNodes[nodePoint.Name] = storedNodePoint
		} else {
			prevNodePoint, found := p.prevNodes[nodePoint.Name]
			if found {
				prevNodes[nodePoint.Name] = prevNodePoint
			}
		}
	}
	p.nodes = newNodes
	p.prevNodes = prevNodes

	// Only count nodes for which metrics can be returned.
	pointsStored.WithLabelValues("node").Set(float64(len(prevNodes)))
}

func (p *storage) storePodMetrics(pods []PodMetricsPoint) {
	p.mu.Lock()
	defer p.mu.Unlock()

	newPods := make(map[apitypes.NamespacedName]map[string]ContainerMetricsPoint, len(pods))
	prevPods := make(map[apitypes.NamespacedName]map[string]ContainerMetricsPoint, len(pods))
	var containerCount int
	for _, podPoint := range pods {
		podIdent := apitypes.NamespacedName{Name: podPoint.Name, Namespace: podPoint.Namespace}
		if _, exists := newPods[podIdent]; exists {
			klog.ErrorS(nil, "Got duplicate pod point", "pod", klog.KRef(podPoint.Namespace, podPoint.Name))
			continue
		}

		newContainers := make(map[string]ContainerMetricsPoint, len(podPoint.Containers))
		prevContainers := make(map[string]ContainerMetricsPoint, len(podPoint.Containers))
		for _, contPoint := range podPoint.Containers {
			if _, exists := newContainers[contPoint.Name]; exists {
				klog.ErrorS(nil, "Got duplicate container point", "container", contPoint.Name, "pod", klog.KRef(podPoint.Namespace, podPoint.Name))
				continue
			}
			newContainers[contPoint.Name] = contPoint

			// If the new point is newer than the one stored for the container, move
			// it to the list of the previous points.
			// This check also prevents from updating the store if the same metric
			// point was scraped twice.
			storedPodPoint, found := p.pods[podIdent]
			if found {
				storedContPoint, found := storedPodPoint[contPoint.Name]
				if found && contPoint.Timestamp.After(storedContPoint.Timestamp) {
					prevContainers[contPoint.Name] = storedContPoint
				} else {
					prevPodPoint, found := p.prevPods[podIdent]
					if found {
						prevContainers[contPoint.Name] = prevPodPoint[contPoint.Name]
					}
				}
			}
		}
		containerPoints := len(prevContainers)
		if containerPoints > 0 {
			prevPods[podIdent] = prevContainers
		}
		newPods[podIdent] = newContainers

		// Only count containers for which metrics can be returned.
		containerCount += containerPoints
	}
	p.pods = newPods
	p.prevPods = prevPods

	pointsStored.WithLabelValues("container").Set(float64(containerCount))
}

func cpuUsageOverTime(metricPoint, prevMetricPoint MetricsPoint) resource.Quantity {
	window := metricPoint.Timestamp.Sub(prevMetricPoint.Timestamp).Seconds()
	cpuUsageScaled := metricPoint.CpuUsage.ScaledValue(-9)
	prevCPUUsageScaled := prevMetricPoint.CpuUsage.ScaledValue(-9)
	cpuUsage := float64(cpuUsageScaled-prevCPUUsageScaled) / window
	return *resource.NewScaledQuantity(int64(cpuUsage), -9)
}
