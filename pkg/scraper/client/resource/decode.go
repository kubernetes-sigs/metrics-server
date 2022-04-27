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
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/prometheus/prometheus/pkg/textparse"
	"github.com/prometheus/prometheus/pkg/timestamp"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/metrics-server/pkg/storage"
)

var (
	nodeCpuUsageMetricName       = []byte("node_cpu_usage_seconds_total")
	nodeMemUsageMetricName       = []byte("node_memory_working_set_bytes")
	containerCpuUsageMetricName  = []byte("container_cpu_usage_seconds_total")
	containerMemUsageMetricName  = []byte("container_memory_working_set_bytes")
	containerStartTimeMetricName = []byte("container_start_time_seconds")
)

func decodeBatch(b []byte, defaultTime time.Time, nodeName string) (*storage.MetricsBatch, error) {
	res := &storage.MetricsBatch{
		Nodes: make(map[string]storage.MetricsPoint),
		Pods:  make(map[apitypes.NamespacedName]storage.PodMetricsPoint),
	}
	node := &storage.MetricsPoint{}
	pods := make(map[apitypes.NamespacedName]storage.PodMetricsPoint)
	parser := textparse.New(b, "")
	var (
		err              error
		defaultTimestamp = timestamp.FromTime(defaultTime)
		et               textparse.Entry
	)
	for {
		if et, err = parser.Next(); err != nil {
			if err == io.EOF {
				break
			} else {
				return nil, fmt.Errorf("failed parsing metrics: %w", err)
			}
		}
		if et != textparse.EntrySeries {
			continue
		}
		timeseries, maybeTimestamp, value := parser.Series()
		if maybeTimestamp == nil {
			maybeTimestamp = &defaultTimestamp
		}
		switch {
		case timeseriesMatchesName(timeseries, nodeCpuUsageMetricName):
			parseNodeCpuUsageMetrics(*maybeTimestamp, value, node)
		case timeseriesMatchesName(timeseries, nodeMemUsageMetricName):
			parseNodeMemUsageMetrics(*maybeTimestamp, value, node)
		case timeseriesMatchesName(timeseries, containerCpuUsageMetricName):
			namespaceName, containerName := parseContainerLabels(timeseries[len(containerCpuUsageMetricName):])
			parseContainerCpuMetrics(namespaceName, containerName, *maybeTimestamp, value, pods)
		case timeseriesMatchesName(timeseries, containerMemUsageMetricName):
			namespaceName, containerName := parseContainerLabels(timeseries[len(containerMemUsageMetricName):])
			parseContainerMemMetrics(namespaceName, containerName, *maybeTimestamp, value, pods)
		case timeseriesMatchesName(timeseries, containerStartTimeMetricName):
			namespaceName, containerName := parseContainerLabels(timeseries[len(containerStartTimeMetricName):])
			parseContainerStartTimeMetrics(namespaceName, containerName, *maybeTimestamp, value, pods)
		default:
			continue
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
	return res, nil
}

func timeseriesMatchesName(ts, name []byte) bool {
	return bytes.HasPrefix(ts, name) && (len(ts) == len(name) || ts[len(name)] == '{')
}

func parseNodeCpuUsageMetrics(timestamp int64, value float64, node *storage.MetricsPoint) {
	// unit of node_cpu_usage_seconds_total is second, need to convert 	i = bytes.Index(labels, podNameTag)
	node.CumulativeCpuUsed = uint64(value * 1e9)
	// unit of timestamp is millisecond, need to convert to nanosecond
	node.Timestamp = time.Unix(0, timestamp*1e6)
}

func parseNodeMemUsageMetrics(timestamp int64, value float64, node *storage.MetricsPoint) {
	node.MemoryUsage = uint64(value)
	// unit of timestamp is millisecond, need to convert to nanosecond
	node.Timestamp = time.Unix(0, timestamp*1e6)
}

func parseContainerCpuMetrics(namespaceName apitypes.NamespacedName, containerName string, timestamp int64, value float64, pods map[apitypes.NamespacedName]storage.PodMetricsPoint) {
	if _, findPod := pods[namespaceName]; !findPod {
		pods[namespaceName] = storage.PodMetricsPoint{Containers: make(map[string]storage.MetricsPoint)}
	}
	if _, findContainer := pods[namespaceName].Containers[containerName]; !findContainer {
		pods[namespaceName].Containers[containerName] = storage.MetricsPoint{}
	}
	// unit of node_cpu_usage_seconds_total is second, need to convert to nanosecond
	containerMetrics := pods[namespaceName].Containers[containerName]
	containerMetrics.CumulativeCpuUsed = uint64(value * 1e9)
	// unit of timestamp is millisecond, need to convert to nanosecond
	containerMetrics.Timestamp = time.Unix(0, timestamp*1e6)
	pods[namespaceName].Containers[containerName] = containerMetrics
}

func parseContainerMemMetrics(namespaceName apitypes.NamespacedName, containerName string, timestamp int64, value float64, pods map[apitypes.NamespacedName]storage.PodMetricsPoint) {
	if _, findPod := pods[namespaceName]; !findPod {
		pods[namespaceName] = storage.PodMetricsPoint{Containers: make(map[string]storage.MetricsPoint)}
	}
	if _, findContainer := pods[namespaceName].Containers[containerName]; !findContainer {
		pods[namespaceName].Containers[containerName] = storage.MetricsPoint{}
	}
	containerMetrics := pods[namespaceName].Containers[containerName]
	containerMetrics.MemoryUsage = uint64(value)
	// unit of timestamp is millisecond, need to convert to nanosecond
	containerMetrics.Timestamp = time.Unix(0, timestamp*1e6)
	pods[namespaceName].Containers[containerName] = containerMetrics
}

func parseContainerStartTimeMetrics(namespaceName apitypes.NamespacedName, containerName string, timestamp int64, value float64, pods map[apitypes.NamespacedName]storage.PodMetricsPoint) {
	if _, findPod := pods[namespaceName]; !findPod {
		pods[namespaceName] = storage.PodMetricsPoint{Containers: make(map[string]storage.MetricsPoint)}
	}
	if _, findContainer := pods[namespaceName].Containers[containerName]; !findContainer {
		pods[namespaceName].Containers[containerName] = storage.MetricsPoint{}
	}
	containerMetrics := pods[namespaceName].Containers[containerName]
	containerMetrics.StartTime = time.Unix(0, int64(value*1e9))
	pods[namespaceName].Containers[containerName] = containerMetrics
}

var (
	containerNameTag = []byte(`container="`)
	podNameTag       = []byte(`pod="`)
	namespaceTag     = []byte(`namespace="`)
)

func parseContainerLabels(labels []byte) (namespaceName apitypes.NamespacedName, containerName string) {
	i := bytes.Index(labels, containerNameTag) + len(containerNameTag)
	j := bytes.IndexByte(labels[i:], '"')
	containerName = string(labels[i : i+j])
	i = bytes.Index(labels, podNameTag) + len(podNameTag)
	j = bytes.IndexByte(labels[i:], '"')
	namespaceName.Name = string(labels[i : i+j])
	i = bytes.Index(labels, namespaceTag) + len(namespaceTag)
	j = bytes.IndexByte(labels[i:], '"')
	namespaceName.Namespace = string(labels[i : i+j])
	return namespaceName, containerName
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
