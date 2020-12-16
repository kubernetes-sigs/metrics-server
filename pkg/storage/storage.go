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
	mu    sync.RWMutex
	nodes map[string]NodeMetricsPoint
	pods  map[apitypes.NamespacedName]PodMetricsPoint
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

		timestamps[i] = api.TimeInfo{
			Timestamp: metricPoint.Timestamp,
			Window:    kubernetesCadvisorWindow,
		}
		resMetrics[i] = corev1.ResourceList{
			corev1.ResourceName(corev1.ResourceCPU):    metricPoint.CpuUsage,
			corev1.ResourceName(corev1.ResourceMemory): metricPoint.MemoryUsage,
		}
	}

	return timestamps, resMetrics, nil
}

func (p *storage) GetContainerMetrics(pods ...apitypes.NamespacedName) ([]api.TimeInfo, [][]metrics.ContainerMetrics, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	timestamps := make([]api.TimeInfo, len(pods))
	resMetrics := make([][]metrics.ContainerMetrics, len(pods))

	for i, pod := range pods {
		metricPoint, present := p.pods[pod]
		if !present {
			continue
		}

		contMetrics := make([]metrics.ContainerMetrics, len(metricPoint.Containers))
		var earliestTS *time.Time
		for i, contPoint := range metricPoint.Containers {
			contMetrics[i] = metrics.ContainerMetrics{
				Name: contPoint.Name,
				Usage: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceCPU):    contPoint.CpuUsage,
					corev1.ResourceName(corev1.ResourceMemory): contPoint.MemoryUsage,
				},
			}
			if earliestTS == nil || earliestTS.After(contPoint.Timestamp) {
				ts := contPoint.Timestamp // copy to avoid loop iteration variable issues
				earliestTS = &ts
			}
		}
		if earliestTS == nil {
			// we had no containers
			earliestTS = &time.Time{}
		}
		timestamps[i] = api.TimeInfo{
			Timestamp: *earliestTS,
			Window:    kubernetesCadvisorWindow,
		}
		resMetrics[i] = contMetrics
	}
	return timestamps, resMetrics, nil
}

func (p *storage) Store(batch *MetricsBatch) {
	newNodes := make(map[string]NodeMetricsPoint, len(batch.Nodes))
	var nodeCount, containerCount int
	for _, nodePoint := range batch.Nodes {
		if _, exists := newNodes[nodePoint.Name]; exists {
			klog.Errorf("duplicate node %s received", nodePoint.Name)
			continue
		}
		nodeCount += 1
		newNodes[nodePoint.Name] = nodePoint
	}

	newPods := make(map[apitypes.NamespacedName]PodMetricsPoint, len(batch.Pods))
	for _, podPoint := range batch.Pods {
		podIdent := apitypes.NamespacedName{Name: podPoint.Name, Namespace: podPoint.Namespace}
		if _, exists := newPods[podIdent]; exists {
			klog.Errorf("duplicate pod %s received", podIdent)
			continue
		}
		containerCount += len(podPoint.Containers)
		newPods[podIdent] = podPoint
	}

	pointsStored.WithLabelValues("node").Set(float64(nodeCount))
	pointsStored.WithLabelValues("container").Set(float64(containerCount))
	p.mu.Lock()
	p.nodes = newNodes
	p.pods = newPods
	p.mu.Unlock()

}
