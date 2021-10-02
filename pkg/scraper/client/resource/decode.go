// Copyright 2021 The Kubernetes Authors.
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

package resource

import (
	"time"

	"github.com/prometheus/common/model"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"sigs.k8s.io/metrics-server/pkg/storage"
)

const (
	nameSpaceMetricName         = "namespace"
	podNameMetricName           = "pod"
	containerNameMetricName     = "container"
	nodeCpuUsageMetricName      = model.LabelValue("node_cpu_usage_seconds_total")
	nodeMemUsageMetricName      = model.LabelValue("node_memory_working_set_bytes")
	containerCpuUsageMetricName = model.LabelValue("container_cpu_usage_seconds_total")
	containerMemUsageMetricName = model.LabelValue("container_memory_working_set_bytes")
)

func decodeBatch(samples []*model.Sample, nodeName string) *storage.MetricsBatch {
	if len(samples) == 0 {
		return nil
	}
	res := &storage.MetricsBatch{
		Nodes: make(map[string]storage.MetricsPoint),
		Pods:  make(map[apitypes.NamespacedName]storage.PodMetricsPoint),
	}
	node := &storage.MetricsPoint{}
	pods := make(map[apitypes.NamespacedName]storage.PodMetricsPoint)
	for _, sample := range samples {
		// parse metrics from sample
		switch sample.Metric[model.MetricNameLabel] {
		case nodeCpuUsageMetricName:
			parseNodeCpuUsageMetrics(sample, node)
		case nodeMemUsageMetricName:
			parseNodeMemUsageMetrics(sample, node)
		case containerCpuUsageMetricName:
			parseContainerCpuMetrics(sample, pods)
		case containerMemUsageMetricName:
			parseContainerMemMetrics(sample, pods)
		}
	}

	if node.Timestamp.IsZero() || node.CumulativeCpuUsed == 0 || node.MemoryUsage == 0 {
		klog.V(1).InfoS("Failed getting complete node metric", "node", nodeName, "metric", node)
		node = nil
	} else {
		res.Nodes[nodeName] = *node
	}

	for podRef, podMetric := range pods {
		if len(podMetric.Containers) != 0 {
			// drop container metrics when Timestamp is zero

			pm := storage.PodMetricsPoint{
				Containers: checkContainerMetrics(podMetric),
			}
			if pm.Containers == nil {
				klog.V(1).InfoS("Failed getting complete Pod metric", "pod", klog.KRef(podRef.Namespace, podRef.Name))
			} else {
				res.Pods[podRef] = pm
			}
		}
	}
	return res
}

func getNamespaceName(sample *model.Sample) apitypes.NamespacedName {
	return apitypes.NamespacedName{Namespace: string(sample.Metric[nameSpaceMetricName]), Name: string(sample.Metric[podNameMetricName])}
}

func parseNodeCpuUsageMetrics(sample *model.Sample, node *storage.MetricsPoint) {
	// unit of node_cpu_usage_seconds_total is second, need to convert to nanosecond
	node.CumulativeCpuUsed = uint64(sample.Value * 1e9)
	if sample.Timestamp != 0 {
		// unit of timestamp is millisecond, need to convert to nanosecond
		node.Timestamp = time.Unix(0, int64(sample.Timestamp*1e6))
	}
}

func parseNodeMemUsageMetrics(sample *model.Sample, node *storage.MetricsPoint) {
	node.MemoryUsage = uint64(sample.Value)
	if node.Timestamp.IsZero() && sample.Timestamp != 0 {
		// unit of timestamp is millisecond, need to convert to nanosecond
		node.Timestamp = time.Unix(0, int64(sample.Timestamp*1e6))
	}
}

func parseContainerCpuMetrics(sample *model.Sample, pods map[apitypes.NamespacedName]storage.PodMetricsPoint) {
	namespaceName := getNamespaceName(sample)
	containerName := string(sample.Metric[containerNameMetricName])
	if _, findPod := pods[namespaceName]; !findPod {
		pods[namespaceName] = storage.PodMetricsPoint{Containers: make(map[string]storage.MetricsPoint)}
	}
	if _, findContainer := pods[namespaceName].Containers[containerName]; !findContainer {
		pods[namespaceName].Containers[containerName] = storage.MetricsPoint{}
	}
	// unit of node_cpu_usage_seconds_total is second, need to convert to nanosecond
	containerMetrics := pods[namespaceName].Containers[containerName]
	containerMetrics.CumulativeCpuUsed = uint64(sample.Value * 1e9)
	if sample.Timestamp != 0 {
		// unit of timestamp is millisecond, need to convert to nanosecond
		containerMetrics.Timestamp = time.Unix(0, int64(sample.Timestamp*1e6))
	}
	pods[namespaceName].Containers[containerName] = containerMetrics
}

func parseContainerMemMetrics(sample *model.Sample, pods map[apitypes.NamespacedName]storage.PodMetricsPoint) {
	namespaceName := getNamespaceName(sample)
	containerName := string(sample.Metric[containerNameMetricName])

	if _, findPod := pods[namespaceName]; !findPod {
		pods[namespaceName] = storage.PodMetricsPoint{Containers: make(map[string]storage.MetricsPoint)}
	}
	if _, findContainer := pods[namespaceName].Containers[containerName]; !findContainer {
		pods[namespaceName].Containers[containerName] = storage.MetricsPoint{}
	}
	containerMetrics := pods[namespaceName].Containers[containerName]
	containerMetrics.MemoryUsage = uint64(sample.Value)
	if containerMetrics.Timestamp.IsZero() && sample.Timestamp != 0 {
		// unit of timestamp is millisecond, need to convert to nanosecond
		containerMetrics.Timestamp = time.Unix(0, int64(sample.Timestamp*1e6))
	}
	pods[namespaceName].Containers[containerName] = containerMetrics
}

func checkContainerMetrics(podMetric storage.PodMetricsPoint) map[string]storage.MetricsPoint {
	podMetrics := make(map[string]storage.MetricsPoint)
	for containerName, containerMetric := range podMetric.Containers {
		if containerMetric != (storage.MetricsPoint{}) {
			// drop metrics when CumulativeCpuUsed or MemoryUsage is zero
			if containerMetric.CumulativeCpuUsed == 0 || containerMetric.MemoryUsage == 0 {
				klog.V(1).InfoS("Failed getting complete container metric", "containerName", containerName, "containerMetric", containerMetric)
				return nil
			} else {
				podMetrics[containerName] = containerMetric
			}
		}
	}
	return podMetrics
}
