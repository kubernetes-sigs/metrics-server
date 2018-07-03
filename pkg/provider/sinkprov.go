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

package provider

import (
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	metrics "k8s.io/metrics/pkg/apis/metrics"

	"github.com/kubernetes-incubator/metrics-server/pkg/sink"
	"github.com/kubernetes-incubator/metrics-server/pkg/sources"
)

// sinkMetricsProvider is a provider.MetricsProvider that also acts as a sink.MetricSink
type sinkMetricsProvider struct {
	mu    sync.RWMutex
	nodes map[string]sources.NodeMetricsPoint
	pods  map[apitypes.NamespacedName]sources.PodMetricsPoint
}

// NewSinkProvider returns a MetricSink that feeds into a MetricsProvider.
func NewSinkProvider() (sink.MetricSink, MetricsProvider) {
	prov := &sinkMetricsProvider{}
	return prov, prov
}

func (p *sinkMetricsProvider) GetNodeMetrics(node string) (time.Time, corev1.ResourceList, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	metricPoint, present := p.nodes[node]
	if !present {
		return time.Time{}, nil, fmt.Errorf("no metrics known for node %q", node)
	}

	return metricPoint.Timestamp, corev1.ResourceList{
		corev1.ResourceName(corev1.ResourceCPU):    metricPoint.CpuUsage,
		corev1.ResourceName(corev1.ResourceMemory): metricPoint.MemoryUsage,
	}, nil
}

func (p *sinkMetricsProvider) GetContainerMetrics(pod apitypes.NamespacedName) (time.Time, []metrics.ContainerMetrics, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	metricPoint, present := p.pods[pod]
	if !present {
		return time.Time{}, nil, fmt.Errorf("no metrics known for pod \"%s/%s\"", pod.Namespace, pod.Name)
	}

	res := make([]metrics.ContainerMetrics, len(metricPoint.Containers))
	latestTS := time.Time{}
	for i, contPoint := range metricPoint.Containers {
		res[i] = metrics.ContainerMetrics{
			Name: contPoint.Name,
			Usage: corev1.ResourceList{
				corev1.ResourceName(corev1.ResourceCPU):    contPoint.CpuUsage,
				corev1.ResourceName(corev1.ResourceMemory): contPoint.MemoryUsage,
			},
		}
		if latestTS.IsZero() || latestTS.Before(contPoint.Timestamp) {
			latestTS = contPoint.Timestamp
		}
	}
	return latestTS, res, nil
}

func (p *sinkMetricsProvider) Receive(batch *sources.MetricsBatch) error {
	newNodes := make(map[string]sources.NodeMetricsPoint, len(batch.Nodes))
	for _, nodePoint := range batch.Nodes {
		if _, exists := newNodes[nodePoint.Name]; exists {
			return fmt.Errorf("duplicate node %s received", nodePoint.Name)
		}
		newNodes[nodePoint.Name] = nodePoint
	}

	newPods := make(map[apitypes.NamespacedName]sources.PodMetricsPoint, len(batch.Pods))
	for _, podPoint := range batch.Pods {
		podIdent := apitypes.NamespacedName{Name: podPoint.Name, Namespace: podPoint.Namespace}
		if _, exists := newPods[podIdent]; exists {
			return fmt.Errorf("duplicate pod %s received", podIdent)
		}
		newPods[podIdent] = podPoint
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.nodes = newNodes
	p.pods = newPods

	return nil
}
