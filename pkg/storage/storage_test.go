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
	"sort"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/component-base/metrics/testutil"
	"k8s.io/metrics/pkg/apis/metrics"

	"sigs.k8s.io/metrics-server/pkg/api"
)

func TestStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "storage Suite")
}

func newMetricsPoint(st time.Time, ts time.Time, cpu, memory int64) MetricsPoint {
	return MetricsPoint{
		StartTime:         st,
		Timestamp:         ts,
		CumulativeCpuUsed: *resource.NewScaledQuantity(cpu, -9),
		MemoryUsage:       *resource.NewMilliQuantity(memory, resource.BinarySI),
	}
}

var _ = Describe("In-memory storage", func() {
	var (
		batch     *MetricsBatch
		prevBatch *MetricsBatch
		storage   *storage
		now       time.Time
	)

	BeforeEach(func() {
		now = time.Now()
		prevTS := now.Add(-10 * time.Second)
		prevBatch = &MetricsBatch{
			Nodes: []NodeMetricsPoint{
				{Name: "node1", MetricsPoint: newMetricsPoint(prevTS, prevTS.Add(100*time.Millisecond), 10, 120)},
				{Name: "node2", MetricsPoint: newMetricsPoint(prevTS, prevTS.Add(200*time.Millisecond), 110, 220)},
				{Name: "node3", MetricsPoint: newMetricsPoint(prevTS, prevTS.Add(300*time.Millisecond), 210, 320)},
			},
			Pods: []PodMetricsPoint{
				{Name: "pod1", Namespace: "ns1", Containers: []ContainerMetricsPoint{
					{Name: "container1", MetricsPoint: newMetricsPoint(prevTS, prevTS.Add(400*time.Millisecond), 310, 420)},
					{Name: "container2", MetricsPoint: newMetricsPoint(prevTS, prevTS.Add(500*time.Millisecond), 410, 520)},
				}},
				{Name: "pod2", Namespace: "ns1", Containers: []ContainerMetricsPoint{
					{Name: "container1", MetricsPoint: newMetricsPoint(prevTS, prevTS.Add(600*time.Millisecond), 510, 620)},
				}},
				{Name: "pod1", Namespace: "ns2", Containers: []ContainerMetricsPoint{
					{Name: "container1", MetricsPoint: newMetricsPoint(prevTS, prevTS.Add(700*time.Millisecond), 610, 720)},
					{Name: "container2", MetricsPoint: newMetricsPoint(prevTS, prevTS.Add(800*time.Millisecond), 710, 820)},
				}},
			},
		}
		batch = &MetricsBatch{
			Nodes: []NodeMetricsPoint{
				{Name: "node1", MetricsPoint: newMetricsPoint(prevTS, now.Add(100*time.Millisecond), 110, 120)},
				{Name: "node2", MetricsPoint: newMetricsPoint(prevTS, now.Add(200*time.Millisecond), 210, 220)},
				{Name: "node3", MetricsPoint: newMetricsPoint(prevTS, now.Add(300*time.Millisecond), 310, 320)},
			},
			Pods: []PodMetricsPoint{
				{Name: "pod1", Namespace: "ns1", Containers: []ContainerMetricsPoint{
					{Name: "container1", MetricsPoint: newMetricsPoint(prevTS, now.Add(400*time.Millisecond), 410, 420)},
					{Name: "container2", MetricsPoint: newMetricsPoint(prevTS, now.Add(500*time.Millisecond), 510, 520)},
				}},
				{Name: "pod2", Namespace: "ns1", Containers: []ContainerMetricsPoint{
					{Name: "container1", MetricsPoint: newMetricsPoint(prevTS, now.Add(600*time.Millisecond), 610, 620)},
				}},
				{Name: "pod1", Namespace: "ns2", Containers: []ContainerMetricsPoint{
					{Name: "container1", MetricsPoint: newMetricsPoint(prevTS, now.Add(700*time.Millisecond), 710, 720)},
					{Name: "container2", MetricsPoint: newMetricsPoint(prevTS, now.Add(800*time.Millisecond), 810, 820)},
				}},
			},
		}

		storage = NewStorage()

		// Store the previous batch to make sure there are enough metric points
		// in the storage to compute CPU usages.
		storage.Store(prevBatch)
	})

	It("should receive batches of metrics", func() {
		By("storing the batch")
		storage.Store(batch)

		By("making sure that the storage contains all nodes received")
		for _, node := range batch.Nodes {
			_, _, err := storage.GetNodeMetrics(node.Name)
			Expect(err).NotTo(HaveOccurred())
		}

		By("making sure that the storage contains all pods received")
		for _, pod := range batch.Pods {
			ts, metrics, err := storage.GetContainerMetrics(apitypes.NamespacedName{
				Name:      pod.Name,
				Namespace: pod.Namespace,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(ts).To(HaveLen(1))
			Expect(metrics).To(HaveLen(1))
		}
	})

	It("should not error out if duplicate nodes were received, with a partial store", func() {
		By("adding a duplicate node to the batch")
		batch.Nodes = append(batch.Nodes, batch.Nodes[0])

		By("storing the batch and checking for an error")
		storage.Store(batch)

		By("making sure none of the data is in the storage")
		for _, node := range batch.Nodes {
			_, res, err := storage.GetNodeMetrics(node.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(ConsistOf(corev1.ResourceList{
				corev1.ResourceName(corev1.ResourceCPU):    *resource.NewScaledQuantity(10, -9),
				corev1.ResourceName(corev1.ResourceMemory): node.MemoryUsage,
			}))
		}
		for _, pod := range batch.Pods {
			_, res, err := storage.GetContainerMetrics(apitypes.NamespacedName{
				Name:      pod.Name,
				Namespace: pod.Namespace,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(res).NotTo(Equal([][]metrics.ContainerMetrics{nil}))
		}
	})

	It("should not error out if duplicate pods were received, with a partial store", func() {
		By("adding a duplicate pod to the batch")
		batch.Pods = append(batch.Pods, batch.Pods[0])

		By("storing and checking for an error")
		storage.Store(batch)

		By("making sure none of the data is in the storage")
		for _, node := range batch.Nodes {
			_, res, err := storage.GetNodeMetrics(node.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(ConsistOf(corev1.ResourceList{
				corev1.ResourceName(corev1.ResourceCPU):    *resource.NewScaledQuantity(10, -9),
				corev1.ResourceName(corev1.ResourceMemory): node.MemoryUsage,
			}))
		}
		for _, pod := range batch.Pods {
			_, res, err := storage.GetContainerMetrics(apitypes.NamespacedName{
				Name:      pod.Name,
				Namespace: pod.Namespace,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(res).NotTo(Equal([][]metrics.ContainerMetrics{nil}))
		}
	})

	It("should not clear storage cache when input is empty batch", func() {
		By("storing a non-empty batch")
		storage.Store(batch)

		By("storing an empty batch, asserting the request fails")
		storage.Store(&MetricsBatch{})

		By("ensuring the storage previous cache value for nodes remains")
		for _, node := range batch.Nodes {
			ts, metrics, err := storage.GetNodeMetrics(node.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(ts).To(HaveLen(1))
			Expect(metrics).To(HaveLen(1))
		}

		By("ensuring the storage previous cache value for pods remains")
		for _, pod := range batch.Pods {
			ts, metrics, err := storage.GetContainerMetrics(apitypes.NamespacedName{
				Name:      pod.Name,
				Namespace: pod.Namespace,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(ts).To(HaveLen(1))
			Expect(metrics).To(HaveLen(1))
		}

	})

	It("should retrieve metrics for all containers in a pod, with overall latest scrape time", func() {
		By("storing and checking for an error")
		storage.Store(batch)

		By("fetching the pod")
		ts, containerMetrics, err := storage.GetContainerMetrics(apitypes.NamespacedName{
			Name:      "pod1",
			Namespace: "ns1",
		})
		Expect(err).NotTo(HaveOccurred())

		By("verifying that the timestamp is the smallest time amongst all containers")
		Expect(ts).To(ConsistOf(api.TimeInfo{Timestamp: now.Add(400 * time.Millisecond), Window: 10 * time.Second}))

		By("verifying that all containers have data")
		sortContainerMetrics(containerMetrics)
		Expect(containerMetrics).To(BeEquivalentTo(
			[][]metrics.ContainerMetrics{
				{
					{
						Name: "container1",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewScaledQuantity(10, -9),
							corev1.ResourceMemory: *resource.NewMilliQuantity(420, resource.BinarySI),
						},
					},
					{
						Name: "container2",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewScaledQuantity(10, -9),
							corev1.ResourceMemory: *resource.NewMilliQuantity(520, resource.BinarySI),
						},
					},
				},
			},
		))
	})

	It("should return nil metrics for missing pods", func() {
		By("storing and checking for an error")
		storage.Store(batch)

		By("fetching the a present pod and a missing pod")
		ts, containerMetrics, err := storage.GetContainerMetrics(apitypes.NamespacedName{
			Name:      "pod1",
			Namespace: "ns1",
		}, apitypes.NamespacedName{
			Name:      "pod2",
			Namespace: "ns42",
		})
		Expect(err).NotTo(HaveOccurred())

		By("verifying that the timestamp is the smallest time amongst all containers")
		Expect(ts).To(Equal([]api.TimeInfo{{Timestamp: now.Add(400 * time.Millisecond), Window: 10 * time.Second}, {}}))

		By("verifying that all present containers have data")
		sortContainerMetrics(containerMetrics)
		Expect(containerMetrics).To(BeEquivalentTo(
			[][]metrics.ContainerMetrics{
				{
					{
						Name: "container1",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewScaledQuantity(10, -9),
							corev1.ResourceMemory: *resource.NewMilliQuantity(420, resource.BinarySI),
						},
					},
					{
						Name: "container2",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewScaledQuantity(10, -9),
							corev1.ResourceMemory: *resource.NewMilliQuantity(520, resource.BinarySI),
						},
					},
				},
				nil,
			},
		))
	})

	It("should return nil metrics when a pod was added in the last scrape", func() {
		newPodPoint := PodMetricsPoint{Name: "pod2", Namespace: "ns2", Containers: []ContainerMetricsPoint{
			{Name: "container1", MetricsPoint: newMetricsPoint(now, now.Add(900*time.Millisecond), 910, 920)},
		}}

		By("adding a new pod to the batch")
		batch.Pods = append(batch.Pods, newPodPoint)

		By("storing the batch")
		storage.Store(batch)

		By("fetching the new pod")
		ts, containerMetrics, err := storage.GetContainerMetrics(
			apitypes.NamespacedName{Name: newPodPoint.Name, Namespace: newPodPoint.Namespace},
		)
		Expect(err).NotTo(HaveOccurred())

		By("verifying that the timestamp is zero")
		Expect(ts).To(Equal([]api.TimeInfo{{}}))

		By("verifying that the container metrics is nil")
		Expect(containerMetrics).To(Equal([][]metrics.ContainerMetrics{nil}))
	})

	It("should return nil metrics when a pod was removed in the last scrape", func() {
		removedPod := apitypes.NamespacedName{Name: batch.Pods[0].Name, Namespace: batch.Pods[0].Namespace}

		By("removing the pod from the batch")
		batch.Pods = batch.Pods[1:]

		By("storing the batch")
		storage.Store(batch)

		By("fetching the removed pod")
		ts, containerMetrics, err := storage.GetContainerMetrics(removedPod)
		Expect(err).NotTo(HaveOccurred())

		By("verifying that the timestamp is zero")
		Expect(ts).To(Equal([]api.TimeInfo{{}}))

		By("verifying that the container metrics is nil")
		Expect(containerMetrics).To(Equal([][]metrics.ContainerMetrics{nil}))
	})

	It("shoudln't update the store if the last 2 pod metric points were equal", func() {
		By("replacing metric points from the new batch by their previous values")
		batch.Pods[0] = prevBatch.Pods[0]

		By("storing the batch")
		storage.Store(batch)

		By("fetching the pods")
		ts, containerMetrics, err := storage.GetContainerMetrics(
			apitypes.NamespacedName{Name: "pod1", Namespace: "ns1"},
			apitypes.NamespacedName{Name: "pod2", Namespace: "ns1"},
		)
		Expect(err).NotTo(HaveOccurred())

		By("verifying that the timestamp of the node with 2 similar metric points is zero")
		Expect(ts).To(Equal([]api.TimeInfo{{}, {Timestamp: now.Add(600 * time.Millisecond), Window: 10 * time.Second}}))

		By("verifying that all present nodes have data except the one with 2 similar metric points")
		Expect(containerMetrics).To(BeEquivalentTo(
			[][]metrics.ContainerMetrics{
				nil,
				{
					{
						Name: "container1",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewScaledQuantity(10, -9),
							corev1.ResourceMemory: *resource.NewMilliQuantity(620, resource.BinarySI),
						},
					},
				},
			},
		))
	})

	It("should retrieve metrics for a node, with overall latest scrape time", func() {
		By("storing and checking for an error")
		storage.Store(batch)

		By("fetching the nodes")
		ts, nodeMetrics, err := storage.GetNodeMetrics("node1", "node2")
		Expect(err).NotTo(HaveOccurred())

		By("verifying that the timestamp is the smallest time amongst all containers")
		Expect(ts).To(Equal([]api.TimeInfo{{Timestamp: now.Add(100 * time.Millisecond), Window: 10 * time.Second}, {Timestamp: now.Add(200 * time.Millisecond), Window: 10 * time.Second}}))

		By("verifying that all nodes have data")
		Expect(nodeMetrics).To(BeEquivalentTo(
			[]corev1.ResourceList{
				{
					corev1.ResourceCPU:    *resource.NewScaledQuantity(10, -9),
					corev1.ResourceMemory: *resource.NewMilliQuantity(120, resource.BinarySI),
				},
				{
					corev1.ResourceCPU:    *resource.NewScaledQuantity(10, -9),
					corev1.ResourceMemory: *resource.NewMilliQuantity(220, resource.BinarySI),
				},
			},
		))
	})

	It("should return nil metrics for missing nodes", func() {
		By("storing and checking for an error")
		storage.Store(batch)

		By("fetching the nodes, plus a missing node")
		ts, nodeMetrics, err := storage.GetNodeMetrics("node1", "node2", "node42")
		Expect(err).NotTo(HaveOccurred())

		By("verifying that the timestamp is the smallest time amongst all containers")
		Expect(ts).To(Equal([]api.TimeInfo{{Timestamp: now.Add(100 * time.Millisecond), Window: 10 * time.Second}, {Timestamp: now.Add(200 * time.Millisecond), Window: 10 * time.Second}, {}}))

		By("verifying that all present nodes have data")
		Expect(nodeMetrics).To(BeEquivalentTo(
			[]corev1.ResourceList{
				{
					corev1.ResourceCPU:    *resource.NewScaledQuantity(10, -9),
					corev1.ResourceMemory: *resource.NewMilliQuantity(120, resource.BinarySI),
				},
				{
					corev1.ResourceCPU:    *resource.NewScaledQuantity(10, -9),
					corev1.ResourceMemory: *resource.NewMilliQuantity(220, resource.BinarySI),
				},
				nil,
			},
		))
	})

	It("should return nil metrics when a node was added in the last scrape", func() {
		newNodePoint := NodeMetricsPoint{
			Name: "node4", MetricsPoint: newMetricsPoint(now, now.Add(400*time.Millisecond), 410, 520),
		}

		By("adding a new node to the batch")
		batch.Nodes = append(batch.Nodes, newNodePoint)

		By("storing the batch")
		storage.Store(batch)

		By("fetching the new node")
		ts, nodeMetrics, err := storage.GetNodeMetrics(newNodePoint.Name)
		Expect(err).NotTo(HaveOccurred())

		By("verifying that the timestamp is zero")
		Expect(ts).To(Equal([]api.TimeInfo{{}}))

		By("verifying that the node metrics is nil")
		Expect(nodeMetrics).To(Equal([]corev1.ResourceList{nil}))
	})

	It("should return nil metrics when a node was removed in the last scrape", func() {
		removedNode := batch.Nodes[0]

		By("removing a node from the batch")
		batch.Nodes = batch.Nodes[1:]

		By("storing the batch")
		storage.Store(batch)

		By("fetching the removed node")
		ts, nodeMetrics, err := storage.GetNodeMetrics(removedNode.Name)
		Expect(err).NotTo(HaveOccurred())

		By("verifying that the timestamp is zero")
		Expect(ts).To(Equal([]api.TimeInfo{{}}))

		By("verifying that the node metrics is nil")
		Expect(nodeMetrics).To(Equal([]corev1.ResourceList{nil}))
	})

	It("shouldn't update the store if the last 2 node metric points were equal", func() {
		By("replacing metric points from the new batch by their previous values")
		batch.Nodes[0] = prevBatch.Nodes[0]

		By("storing the batch")
		storage.Store(batch)

		By("fetching the nodes")
		ts, nodeMetrics, err := storage.GetNodeMetrics("node1", "node2")
		Expect(err).NotTo(HaveOccurred())

		By("verifying that the timestamp of the node with 2 similar metric points is zero")
		Expect(ts).To(Equal([]api.TimeInfo{{}, {Timestamp: now.Add(200 * time.Millisecond), Window: 10 * time.Second}}))

		By("verifying that all present nodes have data except the one with 2 similar metric points")
		Expect(nodeMetrics).To(BeEquivalentTo(
			[]corev1.ResourceList{
				nil,
				{
					corev1.ResourceCPU:    *resource.NewScaledQuantity(10, -9),
					corev1.ResourceMemory: *resource.NewMilliQuantity(220, resource.BinarySI),
				},
			},
		))
	})

	It("should properly calculate metrics", func() {
		pointsStored.Create(nil)
		pointsStored.Reset()

		storage.Store(batch)

		err := testutil.CollectAndCompare(pointsStored, strings.NewReader(`
		# HELP metrics_server_storage_points [ALPHA] Number of metrics points stored.
		# TYPE metrics_server_storage_points gauge
		metrics_server_storage_points{type="node"} 3
		metrics_server_storage_points{type="container"} 5
		`), "metrics_server_storage_points")
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return nil metrics when a node restarted", func() {
		batch.Nodes[0] = NodeMetricsPoint{Name: "node1", MetricsPoint: newMetricsPoint(now, now, 10, 120)}
		storage.Store(batch)
		_, nodeMetrics, err := storage.GetNodeMetrics("node1")
		Expect(err).NotTo(HaveOccurred())
		Expect(nodeMetrics[0]).To(BeNil())
		_, nodeMetrics, err = storage.GetNodeMetrics("node2")
		Expect(err).NotTo(HaveOccurred())
		Expect(nodeMetrics[0]).NotTo(BeNil())
	})

	It("should return nil metrics when a container restarted", func() {
		batch.Pods[0].Containers[0] = ContainerMetricsPoint{Name: "container1", MetricsPoint: newMetricsPoint(now, now.Add(400*time.Millisecond), 310, 420)}
		batch.Pods[0].Containers[1] = ContainerMetricsPoint{Name: "container2", MetricsPoint: newMetricsPoint(now, now.Add(500*time.Millisecond), 410, 520)}
		storage.Store(batch)
		_, containerMetrics, err := storage.GetContainerMetrics(
			apitypes.NamespacedName{Name: "pod1", Namespace: "ns1"},
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(containerMetrics[0]).To(BeNil())
		_, containerMetrics, err = storage.GetContainerMetrics(
			apitypes.NamespacedName{Name: "pod2", Namespace: "ns1"},
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(containerMetrics[0]).NotTo(BeNil())
	})
})

func sortContainerMetrics(cs [][]metrics.ContainerMetrics) {
	for i := range cs {
		sort.Slice(cs[i], func(j, k int) bool {
			return cs[i][j].Name < cs[i][k].Name
		})
	}
}
