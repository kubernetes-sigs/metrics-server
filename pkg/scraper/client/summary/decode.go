// Copyright 2020 The Kubernetes Authors.
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

package summary

import (
	"fmt"

	"k8s.io/klog/v2"

	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/metrics-server/pkg/storage"
)

func decodeBatch(summary *Summary) *storage.MetricsBatch {
	res := &storage.MetricsBatch{
		Nodes: make(map[string]storage.MetricsPoint, 1),
		Pods:  make(map[apitypes.NamespacedName]storage.PodMetricsPoint, len(summary.Pods)),
	}
	decodeNodeStats(&summary.Node, res)
	for _, pod := range summary.Pods {
		decodePodStats(&pod, res)
	}
	return res
}

func decodeNodeStats(nodeStats *NodeStats, batch *storage.MetricsBatch) {
	if nodeStats.StartTime.IsZero() || nodeStats.CPU == nil || nodeStats.CPU.Time.IsZero() {
		// if we can't get a timestamp, assume bad data in general
		klog.V(1).InfoS("Failed getting node metric timestamp", "node", klog.KRef("", nodeStats.NodeName))
		return
	}
	point := storage.MetricsPoint{
		StartTime: nodeStats.StartTime.Time,
		Timestamp: nodeStats.CPU.Time.Time,
	}
	if err := decodeCPU(&point.CumulativeCpuUsed, nodeStats.CPU); err != nil {
		klog.V(1).InfoS("Skipped node CPU metric", "node", klog.KRef("", nodeStats.NodeName), "err", err)
		return
	}
	if err := decodeMemory(&point.MemoryUsage, nodeStats.Memory); err != nil {
		klog.V(1).InfoS("Skipped node memory metric", "node", klog.KRef("", nodeStats.NodeName), "err", err)
		return
	}
	batch.Nodes[nodeStats.NodeName] = point
}

// NB: we explicitly want to discard pods with partial results, since
// the horizontal pod autoscaler takes special action when a pod is missing
// metrics (and zero CPU or memory does not count as "missing metrics")
func decodePodStats(podStats *PodStats, batch *storage.MetricsBatch) {
	// completely overwrite data in the target
	pod := storage.PodMetricsPoint{
		Containers: make(map[string]storage.MetricsPoint, len(podStats.Containers)),
	}
	for _, container := range podStats.Containers {
		if container.StartTime.IsZero() || container.CPU == nil || container.CPU.Time.IsZero() {
			// if we can't get a timestamp, assume bad data in general
			klog.V(1).InfoS("Failed getting container metric timestamp", "containerName", container.Name, "pod", klog.KRef(podStats.PodRef.Namespace, podStats.PodRef.Name))
			return

		}
		point := storage.MetricsPoint{
			StartTime: container.StartTime.Time,
			Timestamp: container.CPU.Time.Time,
		}
		if err := decodeCPU(&point.CumulativeCpuUsed, container.CPU); err != nil {
			klog.V(1).InfoS("Skipped container CPU metric", "containerName", container.Name, "pod", klog.KRef(podStats.PodRef.Namespace, podStats.PodRef.Name), "err", err)
			return
		}
		if err := decodeMemory(&point.MemoryUsage, container.Memory); err != nil {
			klog.V(1).InfoS("Skipped container memory metric", "containerName", container.Name, "pod", klog.KRef(podStats.PodRef.Namespace, podStats.PodRef.Name), "err", err)
			return
		}
		pod.Containers[container.Name] = point
	}
	batch.Pods[apitypes.NamespacedName{Name: podStats.PodRef.Name, Namespace: podStats.PodRef.Namespace}] = pod
}

func decodeCPU(target *uint64, cpuStats *CPUStats) error {
	if cpuStats == nil || cpuStats.UsageCoreNanoSeconds == nil {
		return fmt.Errorf("missing usageCoreNanoSeconds value")
	}

	if *cpuStats.UsageCoreNanoSeconds == 0 {
		return fmt.Errorf("Got UsageCoreNanoSeconds equal zero")
	}
	*target = *cpuStats.UsageCoreNanoSeconds
	return nil
}

func decodeMemory(target *uint64, memStats *MemoryStats) error {
	if memStats == nil || memStats.WorkingSetBytes == nil {
		return fmt.Errorf("missing workingSetBytes value")
	}
	if *memStats.WorkingSetBytes == 0 {
		return fmt.Errorf("Got WorkingSetBytes equal zero")
	}

	*target = *memStats.WorkingSetBytes
	return nil
}
