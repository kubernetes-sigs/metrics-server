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

package scraper

import (
	"context"
	"math"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
	sources "sigs.k8s.io/metrics-server/pkg/storage"
)

var _ = Describe("Summary Source", func() {
	var (
		src        sources.MetricSource
		client     *fakeKubeletClient
		scrapeTime time.Time = time.Now()
		nodeInfo   NodeInfo  = NodeInfo{
			ConnectAddress: "10.0.1.2",
			Name:           "node1",
		}
	)
	BeforeEach(func() {
		client = &fakeKubeletClient{
			metrics: &stats.Summary{
				Node: stats.NodeStats{
					CPU:    cpuStats(100, scrapeTime.Add(100*time.Millisecond)),
					Memory: memStats(200, scrapeTime.Add(200*time.Millisecond)),
				},
				Pods: []stats.PodStats{
					podStats("ns1", "pod1",
						containerStats("container1", 300, 400, scrapeTime.Add(10*time.Millisecond)),
						containerStats("container2", 500, 600, scrapeTime.Add(20*time.Millisecond))),
					podStats("ns1", "pod2",
						containerStats("container1", 700, 800, scrapeTime.Add(30*time.Millisecond))),
					podStats("ns2", "pod1",
						containerStats("container1", 900, 1000, scrapeTime.Add(40*time.Millisecond))),
					podStats("ns3", "pod1",
						containerStats("container1", 1100, 1200, scrapeTime.Add(50*time.Millisecond))),
				},
			},
		}
		src = NewSummaryMetricsSource(nodeInfo, client)
	})

	It("should pass the provided context to the kubelet client to time out requests", func() {
		By("setting up a context with a 1 second timeout")
		ctx, workDone := context.WithTimeout(context.Background(), 1*time.Second)

		By("collecting the batch with a 4 second delay")
		start := time.Now()
		client.delay = 4 * time.Second
		_, err := src.Collect(ctx)
		workDone()

		By("ensuring it timed out with an error after 1 second")
		Expect(time.Since(start)).To(BeNumerically("~", 1*time.Second, 1*time.Millisecond))
		Expect(err).To(HaveOccurred())
	})

	It("should fetch by connection address", func() {
		By("collecting the batch")
		_, err := src.Collect(context.Background())
		Expect(err).NotTo(HaveOccurred())

		By("verifying that it submitted the right host to the client")
		Expect(client.lastHost).To(Equal(nodeInfo.ConnectAddress))
	})

	It("should return the working set and cpu usage for the node, and all pods on the node", func() {
		By("collecting the batch")
		batch, err := src.Collect(context.Background())
		Expect(err).NotTo(HaveOccurred())

		By("verifying that the batch contains the right node data")
		verifyNode(nodeInfo.Name, client.metrics, batch)

		By("verifying that the batch contains the right pod data")
		verifyPods(client.metrics, batch)
	})

	It("should use the scrape time from the CPU, falling back to memory if missing", func() {
		By("removing some times from the data")
		client.metrics.Pods[0].Containers[0].CPU.Time = metav1.Time{}
		client.metrics.Node.CPU.Time = metav1.Time{}

		By("collecting the batch")
		batch, err := src.Collect(context.Background())
		Expect(err).NotTo(HaveOccurred())

		By("verifying that the scrape time is as expected")
		Expect(batch.Nodes[0].Timestamp).To(Equal(client.metrics.Node.Memory.Time.Time))
		Expect(batch.Pods[0].Containers[0].Timestamp).To(Equal(client.metrics.Pods[0].Containers[0].Memory.Time.Time))
		Expect(batch.Pods[1].Containers[0].Timestamp).To(Equal(client.metrics.Pods[1].Containers[0].CPU.Time.Time))
	})

	It("should continue on missing CPU or memory metrics", func() {
		By("removing some data from the raw summary")
		client.metrics.Node.Memory = nil
		client.metrics.Pods[0].Containers[1].CPU = nil
		client.metrics.Pods[1].Containers[0].CPU.UsageNanoCores = nil
		client.metrics.Pods[2].Containers[0].Memory = nil
		client.metrics.Pods[3].Containers[0].Memory.WorkingSetBytes = nil

		By("collecting the batch")
		batch, err := src.Collect(context.Background())
		Expect(err).To(HaveOccurred())

		By("verifying that the batch has all the data, save for what was missing")
		verifyNode(nodeInfo.Name, client.metrics, batch)
		verifyPods(client.metrics, batch)
	})

	It("should handle larger-than-int64 CPU or memory values gracefully", func() {
		By("setting some data in the summary to be above math.MaxInt64")
		plusTen := uint64(math.MaxInt64 + 10)
		plusTwenty := uint64(math.MaxInt64 + 20)
		minusTen := uint64(math.MaxUint64 - 10)
		minusOneHundred := uint64(math.MaxUint64 - 100)

		client.metrics.Node.Memory.WorkingSetBytes = &plusTen // RAM is cheap, right?
		client.metrics.Node.CPU.UsageNanoCores = &plusTwenty  // a mainframe, probably
		client.metrics.Pods[0].Containers[1].CPU.UsageNanoCores = &minusTen
		client.metrics.Pods[1].Containers[0].Memory.WorkingSetBytes = &minusOneHundred

		By("collecting the batch")
		batch, err := src.Collect(context.Background())
		Expect(err).NotTo(HaveOccurred())

		By("verifying that the data is still present, at lower precision")
		nodeMem := *resource.NewScaledQuantity(int64(plusTen/10), 1)
		nodeMem.Format = resource.BinarySI
		podMem := *resource.NewScaledQuantity(int64(minusOneHundred/10), 1)
		podMem.Format = resource.BinarySI
		Expect(batch.Nodes[0].MemoryUsage).To(Equal(nodeMem))
		Expect(batch.Nodes[0].CpuUsage).To(Equal(*resource.NewScaledQuantity(int64(plusTwenty/10), -8)))
		Expect(batch.Pods[0].Containers[1].CpuUsage).To(Equal(*resource.NewScaledQuantity(int64(minusTen/10), -8)))
		Expect(batch.Pods[1].Containers[0].MemoryUsage).To(Equal(podMem))
	})
})

func verifyNode(nodeName string, summary *stats.Summary, batch *sources.MetricsBatch) {
	var cpuUsage, memoryUsage resource.Quantity
	var timestamp time.Time
	if summary.Node.CPU != nil {
		if summary.Node.CPU.UsageNanoCores != nil {
			cpuUsage = *resource.NewScaledQuantity(int64(*summary.Node.CPU.UsageNanoCores), -9)
		}
		timestamp = summary.Node.CPU.Time.Time
	}
	if summary.Node.Memory != nil {
		if summary.Node.Memory.WorkingSetBytes != nil {
			memoryUsage = *resource.NewQuantity(int64(*summary.Node.Memory.WorkingSetBytes), resource.BinarySI)
		}
		if timestamp.IsZero() {
			timestamp = summary.Node.Memory.Time.Time
		}
	}

	Expect(batch.Nodes).To(ConsistOf(
		sources.NodeMetricsPoint{
			Name: nodeName,
			MetricsPoint: sources.MetricsPoint{
				Timestamp:   timestamp,
				CpuUsage:    cpuUsage,
				MemoryUsage: memoryUsage,
			},
		},
	))
}

func verifyPods(summary *stats.Summary, batch *sources.MetricsBatch) {
	var expectedPods []interface{}
	for _, pod := range summary.Pods {
		containers := make([]sources.ContainerMetricsPoint, len(pod.Containers))
		missingData := false
		for i, container := range pod.Containers {
			var cpuUsage, memoryUsage resource.Quantity
			var timestamp time.Time
			if container.CPU == nil || container.CPU.UsageNanoCores == nil {
				missingData = true
				break
			}
			cpuUsage = *resource.NewScaledQuantity(int64(*container.CPU.UsageNanoCores), -9)
			timestamp = container.CPU.Time.Time
			if container.Memory == nil || container.Memory.WorkingSetBytes == nil {
				missingData = true
				break
			}
			memoryUsage = *resource.NewQuantity(int64(*container.Memory.WorkingSetBytes), resource.BinarySI)
			if timestamp.IsZero() {
				timestamp = container.Memory.Time.Time
			}

			containers[i] = sources.ContainerMetricsPoint{
				Name: container.Name,
				MetricsPoint: sources.MetricsPoint{
					Timestamp:   timestamp,
					CpuUsage:    cpuUsage,
					MemoryUsage: memoryUsage,
				},
			}
		}
		if missingData {
			continue
		}
		expectedPods = append(expectedPods, sources.PodMetricsPoint{
			Name:       pod.PodRef.Name,
			Namespace:  pod.PodRef.Namespace,
			Containers: containers,
		})
	}
	Expect(batch.Pods).To(ConsistOf(expectedPods...))
}

func cpuStats(usageNanocores uint64, ts time.Time) *stats.CPUStats {
	return &stats.CPUStats{
		Time:           metav1.Time{ts},
		UsageNanoCores: &usageNanocores,
	}
}

func memStats(workingSetBytes uint64, ts time.Time) *stats.MemoryStats {
	return &stats.MemoryStats{
		Time:            metav1.Time{ts},
		WorkingSetBytes: &workingSetBytes,
	}
}

func podStats(namespace, name string, containers ...stats.ContainerStats) stats.PodStats {
	return stats.PodStats{
		PodRef: stats.PodReference{
			Name:      name,
			Namespace: namespace,
		},
		Containers: containers,
	}
}

func containerStats(name string, cpu, mem uint64, baseTime time.Time) stats.ContainerStats {
	return stats.ContainerStats{
		Name:   name,
		CPU:    cpuStats(cpu, baseTime.Add(2*time.Millisecond)),
		Memory: memStats(mem, baseTime.Add(4*time.Millisecond)),
	}
}
