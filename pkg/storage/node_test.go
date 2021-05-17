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
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/component-base/metrics/testutil"

	"sigs.k8s.io/metrics-server/pkg/api"
)

func TestNodeStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "node storage suite")
}

var _ = Describe("Node storage", func() {
	It("provides node metrics from stored batches", func() {
		s := NewStorage(60 * time.Second)
		nodeStart := time.Now()

		By("storing first batch with node1 metrics")
		s.Store(nodeMetricBatch(NodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(10*time.Second), 10*CoreSecond, 2*MiByte)}))

		By("waiting for second batch before becoming ready and serving metrics")
		Expect(s.Ready()).NotTo(BeTrue())
		checkNodeResponseEmpty(s, "node1")

		By("storing second batch with node1 metrics")
		s.Store(nodeMetricBatch(NodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(20*time.Second), 20*CoreSecond, 3*MiByte)}))

		By("becoming ready and returning metric for node1")
		Expect(s.Ready()).To(BeTrue())
		ts, ms, err := s.GetNodeMetrics("node1")
		Expect(err).NotTo(HaveOccurred())
		Expect(ts).Should(BeEquivalentTo([]api.TimeInfo{{Timestamp: nodeStart.Add(20 * time.Second), Window: 10 * time.Second}}))
		Expect(ms).Should(BeEquivalentTo(
			[]v1.ResourceList{
				{
					v1.ResourceCPU:    *resource.NewScaledQuantity(CoreSecond, -9),
					v1.ResourceMemory: *resource.NewQuantity(3*MiByte, resource.BinarySI),
				},
			},
		))
		By("return empty result for not stored node2")
		checkNodeResponseEmpty(s, "node2")

		By("storing third batch without metrics")
		s.Store(nodeMetricBatch())

		By("return empty result for node1")
		checkNodeResponseEmpty(s, "node1")
	})
	It("should handle duplicate node", func() {
		s := NewStorage(60 * time.Second)
		nodeStart := time.Now()

		By("store first batch")
		s.Store(nodeMetricBatch(
			NodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(10*time.Second), 10*CoreSecond, 3*MiByte)},
			NodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(10*time.Second), 10*CoreSecond, 3*MiByte)},
		))

		By("store second batch")
		s.Store(nodeMetricBatch(
			NodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(20*time.Second), 21*CoreSecond, 4*MiByte)},
			NodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(20*time.Second), 21*CoreSecond, 4*MiByte)},
		))

		By("becoming ready and returning correct metric values")
		Expect(s.Ready()).To(BeTrue())
		ts, ms, err := s.GetNodeMetrics("node1")
		Expect(err).NotTo(HaveOccurred())
		Expect(ts).Should(BeEquivalentTo([]api.TimeInfo{{Timestamp: nodeStart.Add(20 * time.Second), Window: 10 * time.Second}}))
		Expect(ms).Should(BeEquivalentTo(
			[]v1.ResourceList{
				{
					v1.ResourceCPU:    *resource.NewScaledQuantity(1.1*CoreSecond, -9),
					v1.ResourceMemory: *resource.NewQuantity(4*MiByte, resource.BinarySI),
				},
			},
		))
	})
	It("handle repeated node metric point", func() {
		s := NewStorage(60 * time.Second)
		nodeStart := time.Now()

		By("storing first batch with node1 metrics")
		batch := nodeMetricBatch(NodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(10*time.Second), 10*CoreSecond, 2*MiByte)})
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
		s.Store(nodeMetricBatch(NodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(10*time.Second), 10*CoreSecond, 2*MiByte)}))

		err = testutil.CollectAndCompare(pointsStored, strings.NewReader(`
		# HELP metrics_server_storage_points [ALPHA] Number of metrics points stored.
		# TYPE metrics_server_storage_points gauge
		metrics_server_storage_points{type="container"} 0
		metrics_server_storage_points{type="node"} 0
		`), "metrics_server_storage_points")
		Expect(err).NotTo(HaveOccurred())

		By("storing second batch with node1 metrics")
		s.Store(nodeMetricBatch(NodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(20*time.Second), 21*CoreSecond, 3*MiByte)}))

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
		s.Store(nodeMetricBatch(NodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(10*time.Second), 10*CoreSecond, 2*MiByte)}))

		By("storing second batch with node1 start time after previous batch")
		s.Store(nodeMetricBatch(NodeMetricsPoint{"node1", newMetricsPoint(nodeStart.Add(15*time.Second), nodeStart.Add(20*time.Second), 5*CoreSecond, 3*MiByte)}))

		By("return empty result for restarted node1")
		checkNodeResponseEmpty(s, "node1")
	})
	It("should return empty node metrics if decreased data point reported", func() {
		s := NewStorage(60 * time.Second)
		nodeStart := time.Now()

		By("storing previous metrics")
		s.Store(nodeMetricBatch(NodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(15*time.Second), 50*CoreSecond, 3*MiByte)}))

		By("storing CPU usage decreased last metrics")
		s.Store(nodeMetricBatch(NodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(25*time.Second), 10*CoreSecond, 5*MiByte)}))

		By("should get empty metrics when cpu metrics decrease")
		checkNodeResponseEmpty(s, "node1")
	})
	It("should handle metrics older then prev", func() {
		s := NewStorage(60 * time.Second)
		nodeStart := time.Now()

		By("storing previous metrics")
		s.Store(nodeMetricBatch(NodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(15*time.Second), 10*CoreSecond, 3*MiByte)}))

		By("storing last metrics")
		s.Store(nodeMetricBatch(NodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(35*time.Second), 50*CoreSecond, 5*MiByte)}))

		By("Storing new metrics older then previous")
		s.Store(nodeMetricBatch(NodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(5*time.Second), 6*CoreSecond, 2*MiByte)}))

		By("should get empty metrics after stored older metrics than previous")
		checkNodeResponseEmpty(s, "node1")
	})

	It("should handle metrics prev.ts < newNode.ts < last.ts", func() {
		s := NewStorage(60 * time.Second)
		nodeStart := time.Now()

		By("storing previous metrics")
		s.Store(nodeMetricBatch(NodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(15*time.Second), 10*CoreSecond, 1*MiByte)}))

		By("storing last metrics")
		s.Store(nodeMetricBatch(NodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(35*time.Second), 50*CoreSecond, 4*MiByte)}))

		By("Storing new metrics prev.ts < node.ts < last.ts")
		s.Store(nodeMetricBatch(NodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(25*time.Second), 35*CoreSecond, 2*MiByte)}))

		By("should get non-empty metrics after stored older metrics than previous")
		ts, ms, err := s.GetNodeMetrics("node1")
		Expect(err).NotTo(HaveOccurred())
		Expect(ts).Should(BeEquivalentTo([]api.TimeInfo{{Timestamp: nodeStart.Add(25 * time.Second), Window: 10 * time.Second}}))
		Expect(ms).Should(BeEquivalentTo(
			[]v1.ResourceList{
				{
					v1.ResourceCPU:    *resource.NewScaledQuantity(2.5*CoreSecond, -9),
					v1.ResourceMemory: *resource.NewQuantity(2*MiByte, resource.BinarySI),
				},
			},
		))
	})
})

func checkNodeResponseEmpty(s *storage, nodes ...string) {
	ts, ms, err := s.GetNodeMetrics(nodes...)
	Expect(err).NotTo(HaveOccurred())
	Expect(ts).To(Equal(make([]api.TimeInfo, len(nodes))))
	Expect(ms).To(Equal(make([]v1.ResourceList, len(nodes))))
}

func nodeMetricBatch(nodes ...NodeMetricsPoint) *MetricsBatch {
	return &MetricsBatch{
		Nodes: nodes,
	}
}
