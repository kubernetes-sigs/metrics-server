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

// kubernetesCadvisorWindow is the max window used by cAdvisor for calculating
// CPU usage rate.  While it can vary, it's no more than this number, but may be
// as low as half this number (when working with no backoff).  It would be really
// nice if the kubelet told us this in the summary API...
var kubernetesCadvisorWindow = 30 * time.Second

// storage is a thread save storage for node and pod metrics
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

// TODO(directxman12): figure out what the right value is for "window" --
// we don't get the actual window from cAdvisor, so we could just
// plumb down metric resolution, but that wouldn't be actually correct.
func (p *storage) GetNodeMetrics(nodes ...string) ([]api.TimeInfo, []corev1.ResourceList, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	timestamps := make([]api.TimeInfo, len(nodes))
	resMetrics := make([]corev1.ResourceList, len(nodes))
	for i, node := range nodes {
		metricPoint, present := p.nodes[node]
		if !present {
			continue
		}

		prevMetricPoint := p.prevNodes[node]
		if prevMetricPoint.Timestamp.IsZero() {
			continue
		}

		cpuUsage := cpuUsageOverTime(metricPoint.MetricsPoint, prevMetricPoint.MetricsPoint)
		resMetrics[i] = corev1.ResourceList{
			corev1.ResourceName(corev1.ResourceCPU):    cpuUsage,
			corev1.ResourceName(corev1.ResourceMemory): metricPoint.MemoryUsage,
		}
		timestamps[i] = api.TimeInfo{
			Timestamp: metricPoint.Timestamp,
			Window:    kubernetesCadvisorWindow,
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
		contPoints, present := p.pods[pod]
		if !present {
			continue
		}

		contMetrics := make([]metrics.ContainerMetrics, len(contPoints))
		var earliestTS time.Time
		var contIdx int
		for contName, contPoint := range contPoints {
			prevContPoint := p.prevPods[pod][contName]
			if prevContPoint.Timestamp.IsZero() {
				continue
			}

			cpuUsage := cpuUsageOverTime(contPoint.MetricsPoint, prevContPoint.MetricsPoint)
			contMetrics[contIdx] = metrics.ContainerMetrics{
				Name: contPoint.Name,
				Usage: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    cpuUsage,
					corev1.ResourceName(corev1.ResourceMemory): contPoint.MemoryUsage,
				},
			}
			if earliestTS.IsZero() || earliestTS.After(contPoint.Timestamp) {
				earliestTS = contPoint.Timestamp
			}
			contIdx++
		}
		resMetrics[podIdx] = contMetrics
		timestamps[podIdx] = api.TimeInfo{
			Timestamp: earliestTS,
			Window:    kubernetesCadvisorWindow,
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
	var nodeCount int
	for _, nodePoint := range nodes {
		if _, exists := newNodes[nodePoint.Name]; exists {
			klog.Errorf("duplicate node %s received", nodePoint.Name)
			continue
		}
		newNodes[nodePoint.Name] = nodePoint

		if nodePoint.Timestamp.After(p.nodes[nodePoint.Name].Timestamp) {
			prevNodes[nodePoint.Name] = p.nodes[nodePoint.Name]
		} else {
			prevNodes[nodePoint.Name] = p.prevNodes[nodePoint.Name]
		}

		nodeCount++
	}
	p.nodes = newNodes
	p.prevNodes = prevNodes

	pointsStored.WithLabelValues("node").Set(float64(nodeCount))
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
			klog.Errorf("duplicate pod %s received", podIdent)
			continue
		}

		newContainers := make(map[string]ContainerMetricsPoint, len(podPoint.Containers))
		prevContainers := make(map[string]ContainerMetricsPoint, len(podPoint.Containers))
		for _, contPoint := range podPoint.Containers {
			if _, exists := newContainers[contPoint.Name]; exists {
				klog.Errorf("duplicate container %s received", contPoint.Name)
				continue
			}
			newContainers[contPoint.Name] = contPoint

			if contPoint.Timestamp.After(p.pods[podIdent][contPoint.Name].Timestamp) {
				prevContainers[contPoint.Name] = p.pods[podIdent][contPoint.Name]
			} else {
				prevContainers[contPoint.Name] = p.prevPods[podIdent][contPoint.Name]
			}

			containerCount++
		}
		newPods[podIdent] = newContainers
		prevPods[podIdent] = prevContainers
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
