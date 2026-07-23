// Copyright 2026 The Kubernetes Authors.
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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apitypes "k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Storage readiness", func() {
	It("is not ready when only node metrics are stored", func() {
		s := NewStorage(60 * time.Second)
		nodeStart := time.Now()

		By("storing two node metric batches without any pod metrics")
		s.Store(nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(10*time.Second), 10*CoreSecond, 2*MiByte)}))
		Expect(s.Ready()).To(BeFalse())
		s.Store(nodeMetricBatch(nodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(20*time.Second), 20*CoreSecond, 3*MiByte)}))
		Expect(s.Ready()).To(BeFalse())
	})

	It("is not ready when only pod metrics are stored", func() {
		s := NewStorage(60 * time.Second)
		containerStart := time.Now()
		podRef := apitypes.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("storing two pod metric batches without any node metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(120*time.Second), 1*CoreSecond, 4*MiByte)})))
		Expect(s.Ready()).To(BeFalse())
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(125*time.Second), 6*CoreSecond, 5*MiByte)})))
		Expect(s.Ready()).To(BeFalse())
	})

	It("is ready when both node and pod metrics are stored", func() {
		s := NewStorage(60 * time.Second)
		nodeStart := time.Now()
		containerStart := nodeStart
		podRef := apitypes.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("storing first combined batch with both node and pod metrics")
		s.Store(metricsBatch(
			nodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(10*time.Second), 10*CoreSecond, 2*MiByte)},
			podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(120*time.Second), 1*CoreSecond, 4*MiByte)}),
		))
		Expect(s.Ready()).To(BeFalse())

		By("storing second combined batch")
		s.Store(metricsBatch(
			nodeMetricsPoint{"node1", newMetricsPoint(nodeStart, nodeStart.Add(20*time.Second), 20*CoreSecond, 3*MiByte)},
			podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(125*time.Second), 6*CoreSecond, 5*MiByte)}),
		))
		Expect(s.Ready()).To(BeTrue())
	})
})

func metricsBatch(node nodeMetricsPoint, pod podMetricsPoint) *MetricsBatch {
	return &MetricsBatch{
		Nodes: map[string]MetricsPoint{node.Name: node.MetricsPoint},
		Pods:  map[apitypes.NamespacedName]PodMetricsPoint{pod.NamespacedName: pod.PodMetricsPoint},
	}
}
