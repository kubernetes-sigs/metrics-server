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
	"math"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

func TestDecode(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Decode Suite")
}

var _ = Describe("Decode", func() {
	var (
		summary *stats.Summary
	)
	BeforeEach(func() {
		scrapeTime := time.Now()
		summary = &stats.Summary{
			Node: stats.NodeStats{
				NodeName: "node1",
				CPU:      cpuStats(100, scrapeTime.Add(100*time.Millisecond)),
				Memory:   memStats(200, scrapeTime.Add(200*time.Millisecond)),
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
		}
	})

	It("should use the decode time from the CPU, falling back to memory if missing", func() {
		By("removing some times from the data")
		summary.Pods[0].Containers[0].CPU.Time = metav1.Time{}
		summary.Node.CPU.Time = metav1.Time{}

		By("decoding")
		batch := decodeBatch(summary)

		By("verifying that the scrape time is as expected")
		Expect(batch.Nodes[0].Timestamp).To(Equal(summary.Node.Memory.Time.Time))
		Expect(batch.Pods[0].Containers[0].Timestamp).To(Equal(summary.Pods[0].Containers[0].Memory.Time.Time))
		Expect(batch.Pods[1].Containers[0].Timestamp).To(Equal(summary.Pods[1].Containers[0].CPU.Time.Time))
	})

	It("should continue on missing CPU or memory metrics", func() {
		By("removing some data from the raw summary")
		summary.Node.Memory = nil
		summary.Pods[0].Containers[1].CPU = nil
		summary.Pods[1].Containers[0].CPU.UsageNanoCores = nil
		summary.Pods[2].Containers[0].Memory = nil
		summary.Pods[3].Containers[0].Memory.WorkingSetBytes = nil

		By("decoding")
		batch := decodeBatch(summary)

		By("verifying that the batch has all the data, save for what was missing")
		Expect(batch.Pods).To(HaveLen(0))
		Expect(batch.Nodes).To(HaveLen(0))
	})

	It("should handle larger-than-int64 CPU or memory values gracefully", func() {
		By("setting some data in the summary to be above math.MaxInt64")
		plusTen := uint64(math.MaxInt64 + 10)
		plusTwenty := uint64(math.MaxInt64 + 20)
		minusTen := uint64(math.MaxUint64 - 10)
		minusOneHundred := uint64(math.MaxUint64 - 100)

		summary.Node.Memory.WorkingSetBytes = &plusTen // RAM is cheap, right?
		summary.Node.CPU.UsageNanoCores = &plusTwenty  // a mainframe, probably
		summary.Pods[0].Containers[1].CPU.UsageNanoCores = &minusTen
		summary.Pods[1].Containers[0].Memory.WorkingSetBytes = &minusOneHundred

		By("decoding")
		batch := decodeBatch(summary)

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

func cpuStats(usageNanocores uint64, ts time.Time) *stats.CPUStats {
	return &stats.CPUStats{
		Time:           metav1.Time{Time: ts},
		UsageNanoCores: &usageNanocores,
	}
}

func memStats(wssBytes uint64, ts time.Time) *stats.MemoryStats {
	return &stats.MemoryStats{
		Time:            metav1.Time{Time: ts},
		WorkingSetBytes: &wssBytes,
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
