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
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/metrics/testutil"
)

var _ = Describe("Node storage", func() {
	It("provides node metrics from stored batches", func() {
		s := NewStorage(60 * time.Second)
		nodeStart := time.Now()

		By("storing first batch with node1 metrics")
		s.Store(nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(10*time.Second), 10*CoreSecond, 2*MiByte)}))

		By("waiting for second batch before becoming ready and serving metrics")
		Expect(s.Ready()).NotTo(BeTrue())
		checkNodeResponseEmpty(s, "node1")

		By("storing second batch with node1 metrics")
		s.Store(nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(20*time.Second), 20*CoreSecond, 3*MiByte)}))

		By("becoming ready and returning metric for node1")
		Expect(s.Ready()).To(BeTrue())
		ms, err := s.GetNodeMetrics(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(ms).To(HaveLen(1))
		Expect(ms[0].Timestamp.Time).Should(BeEquivalentTo(nodeStart.Add(20 * time.Second)))
		Expect(ms[0].Window.Duration).Should(BeEquivalentTo(10 * time.Second))
		Expect(ms[0].Usage).Should(BeEquivalentTo(
			corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewScaledQuantity(CoreSecond, -9),
				corev1.ResourceMemory: *resource.NewQuantity(3*MiByte, resource.BinarySI),
			},
		))
		By("return empty result for not stored node2")
		checkNodeResponseEmpty(s, "node2")

		By("storing third batch without metrics")
		s.Store(nodeMetricBatch())

		By("return empty result for node1")
		checkNodeResponseEmpty(s, "node1")
	})
	It("handle repeated node metric point", func() {
		s := NewStorage(60 * time.Second)
		nodeStart := time.Now()

		By("storing first batch with node1 metrics")
		batch := nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(10*time.Second), 10*CoreSecond, 2*MiByte)})
		s.Store(batch)
		By("storing second batch exactly same metric")
		s.Store(batch)

		By("should not be ready and return empty result for node1")
		checkNodeResponseEmpty(s, "node1")
		Expect(s.Ready()).NotTo(BeTrue())
	})
	It("exposes correct node metrics", func() {
		pointsStored.Create(nil)
		pointsStored.Reset()
		s := NewStorage(60 * time.Second)
		nodeStart := time.Now()

		err := testutil.CollectAndCompare(pointsStored, strings.NewReader(`
		`), "metrics_server_storage_points")
		Expect(err).NotTo(HaveOccurred())

		By("storing first batch with node1 metrics")
		s.Store(nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(10*time.Second), 10*CoreSecond, 2*MiByte)}))

		err = testutil.CollectAndCompare(pointsStored, strings.NewReader(`
		# HELP metrics_server_storage_points [ALPHA] Number of metrics points stored.
		# TYPE metrics_server_storage_points gauge
		metrics_server_storage_points{type="container"} 0
		metrics_server_storage_points{type="node"} 0
		`), "metrics_server_storage_points")
		Expect(err).NotTo(HaveOccurred())

		By("storing second batch with node1 metrics")
		s.Store(nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(20*time.Second), 21*CoreSecond, 3*MiByte)}))

		err = testutil.CollectAndCompare(pointsStored, strings.NewReader(`
		# HELP metrics_server_storage_points [ALPHA] Number of metrics points stored.
		# TYPE metrics_server_storage_points gauge
		metrics_server_storage_points{type="container"} 0
		metrics_server_storage_points{type="node"} 1
		`), "metrics_server_storage_points")
		Expect(err).NotTo(HaveOccurred())
	})
	It("should detect node restart and skip metric", func() {
		s := NewStorage(60 * time.Second)
		nodeStart := time.Now()

		By("storing first batch with node1 metrics")
		s.Store(nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(10*time.Second), 10*CoreSecond, 2*MiByte)}))

		By("storing second batch with node1 start time after previous batch")
		s.Store(nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(nodeStart.Add(15*time.Second), nodeStart.Add(20*time.Second), 5*CoreSecond, 3*MiByte)}))

		By("return empty result for restarted node1")
		checkNodeResponseEmpty(s, "node1")
	})
	It("should return empty node metrics if decreased data point reported", func() {
		s := NewStorage(60 * time.Second)
		nodeStart := time.Now()

		By("storing previous metrics")
		s.Store(nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(15*time.Second), 50*CoreSecond, 3*MiByte)}))

		By("storing CPU usage decreased last metrics")
		s.Store(nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(25*time.Second), 10*CoreSecond, 5*MiByte)}))

		By("should get empty metrics when cpu metrics decrease")
		checkNodeResponseEmpty(s, "node1")
	})
	It("should handle metrics older than prev", func() {
		s := NewStorage(60 * time.Second)
		nodeStart := time.Now()

		By("storing previous metrics")
		s.Store(nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(15*time.Second), 10*CoreSecond, 3*MiByte)}))

		By("storing last metrics")
		s.Store(nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(35*time.Second), 50*CoreSecond, 5*MiByte)}))

		By("Storing new metrics older than previous")
		s.Store(nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(5*time.Second), 6*CoreSecond, 2*MiByte)}))

		By("should get empty metrics after stored older metrics than previous")
		checkNodeResponseEmpty(s, "node1")
	})

	It("should handle metrics prev.ts < newNode.ts < last.ts", func() {
		s := NewStorage(60 * time.Second)
		nodeStart := time.Now()

		By("storing previous metrics")
		s.Store(nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(15*time.Second), 10*CoreSecond, 1*MiByte)}))

		By("storing last metrics")
		s.Store(nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(35*time.Second), 50*CoreSecond, 4*MiByte)}))

		By("Storing new metrics prev.ts < node.ts < last.ts")
		s.Store(nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(25*time.Second), 35*CoreSecond, 2*MiByte)}))

		By("should get non-empty metrics after stored older metrics than previous")
		ms, err := s.GetNodeMetrics(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(ms).Should(HaveLen(1))
		Expect(ms[0].Timestamp.Time).Should(BeEquivalentTo(nodeStart.Add(25 * time.Second)))
		Expect(ms[0].Window.Duration).Should(BeEquivalentTo(10 * time.Second))
		Expect(ms[0].Usage).Should(BeEquivalentTo(
			corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewScaledQuantity(2.5*CoreSecond, -9),
				corev1.ResourceMemory: *resource.NewQuantity(2*MiByte, resource.BinarySI),
			},
		))
	})

	It("provides node metrics from stored batches when StartTime is zero", func() {
		s := NewStorage(60 * time.Second)
		nodeStart := time.Now()

		By("storing first batch with node1 metrics")
		s.Store(nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(time.Time{}, nodeStart.Add(10*time.Second), 10*CoreSecond, 2*MiByte)}))

		By("waiting for second batch before becoming ready and serving metrics")
		Expect(s.Ready()).NotTo(BeTrue())
		checkNodeResponseEmpty(s, "node1")

		By("storing second batch with node1 metrics")
		s.Store(nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(time.Time{}, nodeStart.Add(20*time.Second), 20*CoreSecond, 3*MiByte)}))

		By("becoming ready and returning metric for node1")
		Expect(s.Ready()).To(BeTrue())
		ms, err := s.GetNodeMetrics(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(ms).Should(HaveLen(1))
		Expect(ms[0].Timestamp.Time).Should(BeEquivalentTo(nodeStart.Add(20 * time.Second)))
		Expect(ms[0].Window.Duration).Should(BeEquivalentTo(10 * time.Second))
		Expect(ms[0].Usage).Should(BeEquivalentTo(
			corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewScaledQuantity(CoreSecond, -9),
				corev1.ResourceMemory: *resource.NewQuantity(3*MiByte, resource.BinarySI),
			},
		))
	})

})

func checkNodeResponseEmpty(s *storage, names ...string) {
	nodes := []*corev1.Node{}
	for _, name := range names {
		nodes = append(nodes, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}})
	}
	ms, err := s.GetNodeMetrics(nodes...)
	Expect(err).NotTo(HaveOccurred())
	Expect(ms).To(HaveLen(0))
}

func nodeMetricBatch(nodes ...nodeMetricsPoint) *MetricsBatch {
	batch := &MetricsBatch{
		Nodes: make(map[string]MetricsPoint, len(nodes)),
	}
	for _, node := range nodes {
		batch.Nodes[node.Name] = node.MetricsPoint
	}
	return batch
}

type nodeMetricsPoint struct {
	Name string
	MetricsPoint
}
