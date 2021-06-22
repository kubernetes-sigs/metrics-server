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
		Nodes: make(map[string]storage.NodeMetricsPoint),
		Pods:  make(map[apitypes.NamespacedName]storage.PodMetricsPoint),
	}
	node := &storage.NodeMetricsPoint{}
	pods := make(map[apitypes.NamespacedName]map[string]*storage.ContainerMetricsPoint)
	for _, sample := range samples {
		//parse metrics from sample
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
		node.Name = nodeName
		res.Nodes[nodeName] = *node
	}

	for podRef, podMetric := range pods {
		if len(podMetric) != 0 {
			//drop container metrics when Timestamp is zero

			pm := storage.PodMetricsPoint{
				Name:       podRef.Name,
				Namespace:  podRef.Namespace,
				Containers: checkContainerMetricsTimestamp(podMetric),
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

func parseNodeCpuUsageMetrics(sample *model.Sample, node *storage.NodeMetricsPoint) {
	//unit of node_cpu_usage_seconds_total is second, need to convert to nanosecond
	node.CumulativeCpuUsed = uint64(sample.Value * 1e9)
	if sample.Timestamp != 0 {
		//unit of timestamp is millisecond, need to convert to nanosecond
		node.Timestamp = time.Unix(0, int64(sample.Timestamp*1e6))
	}
}

func parseNodeMemUsageMetrics(sample *model.Sample, node *storage.NodeMetricsPoint) {
	node.MemoryUsage = uint64(sample.Value)
	if node.Timestamp.IsZero() && sample.Timestamp != 0 {
		//unit of timestamp is millisecond, need to convert to nanosecond
		node.Timestamp = time.Unix(0, int64(sample.Timestamp*1e6))
	}
}

func parseContainerCpuMetrics(sample *model.Sample, pods map[apitypes.NamespacedName]map[string]*storage.ContainerMetricsPoint) {
	namespaceName := getNamespaceName(sample)
	containerName := string(sample.Metric[containerNameMetricName])
	var pod map[string]*storage.ContainerMetricsPoint
	var findPod bool
	pod, findPod = pods[namespaceName]
	if !findPod {
		pod = make(map[string]*storage.ContainerMetricsPoint)
		pods[namespaceName] = pod
	}
	var container *storage.ContainerMetricsPoint
	var findContainer bool
	if container, findContainer = pod[containerName]; !findContainer {
		container = &storage.ContainerMetricsPoint{}
		pods[namespaceName][containerName] = container
	}
	container.Name = containerName
	//unit of node_cpu_usage_seconds_total is second, need to convert to nanosecond
	container.CumulativeCpuUsed = uint64(sample.Value * 1e9)
	if sample.Timestamp != 0 {
		//unit of timestamp is millisecond, need to convert to nanosecond
		container.Timestamp = time.Unix(0, int64(sample.Timestamp*1e6))
	}
}

func parseContainerMemMetrics(sample *model.Sample, pods map[apitypes.NamespacedName]map[string]*storage.ContainerMetricsPoint) {
	namespaceName := getNamespaceName(sample)
	containerName := string(sample.Metric[containerNameMetricName])
	var pod map[string]*storage.ContainerMetricsPoint
	var findPod bool
	pod, findPod = pods[namespaceName]
	if !findPod {
		pod = make(map[string]*storage.ContainerMetricsPoint)
		pods[namespaceName] = pod
	}
	var container *storage.ContainerMetricsPoint
	var findContainer bool
	if container, findContainer = pod[containerName]; !findContainer {
		container = &storage.ContainerMetricsPoint{}
		pods[namespaceName][containerName] = container
	}
	container.Name = containerName
	container.MemoryUsage = uint64(sample.Value)
	if container.Timestamp.IsZero() && sample.Timestamp != 0 {
		//unit of timestamp is millisecond, need to convert to nanosecond
		container.Timestamp = time.Unix(0, int64(sample.Timestamp*1e6))
	}
}

func checkContainerMetricsTimestamp(podMetric map[string]*storage.ContainerMetricsPoint) map[string]storage.ContainerMetricsPoint {
	podMetrics := make(map[string]storage.ContainerMetricsPoint)
	for containerName, containerMetric := range podMetric {
		if *containerMetric != (storage.ContainerMetricsPoint{}) {
			//drop metrics when Timestamp is zero
			if containerMetric.Timestamp.IsZero() || containerMetric.CumulativeCpuUsed == 0 || containerMetric.MemoryUsage == 0 {
				klog.V(1).InfoS("Failed getting complete container metric", "containerName", containerMetric.Name, "containerMetric", *containerMetric)
				return nil
			} else {
				podMetrics[containerName] = *containerMetric
			}
		}
	}
	return podMetrics
}
