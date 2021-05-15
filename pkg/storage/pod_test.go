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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/component-base/metrics/testutil"
	"sigs.k8s.io/metrics-server/pkg/api"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/metrics/pkg/apis/metrics"
)

func TestPodStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "pod storage Suite")
}

var _ = Describe("Pod storage", func() {
	It("provides pod metrics from stored batches", func() {
		s := NewStorage()
		containerStart := time.Now()
		podRef := types.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("storing first batch with pod1 metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, ContainerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(10*time.Second), 1*CoreSecond, 4*MiByte)})))

		By("waiting for second batch before serving metrics")
		Expect(s.Ready()).NotTo(BeTrue())
		checkPodResponseEmpty(s, podRef)

		By("storing second batch with pod1 metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, ContainerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(15*time.Second), 6*CoreSecond, 5*MiByte)})))

		By("returning metric for pod1")
		Expect(s.Ready()).NotTo(BeTrue())
		ts, ms, err := s.GetPodMetrics(podRef)
		Expect(err).NotTo(HaveOccurred())
		Expect(ts).To(HaveLen(1))
		Expect(ts[0]).To(Equal(api.TimeInfo{
			Timestamp: containerStart.Add(15 * time.Second),
			Window:    5 * time.Second,
		}))
		Expect(ms).To(HaveLen(1))
		Expect(ms[0]).To(HaveLen(1))
		Expect(ms[0][0]).To(Equal(metrics.ContainerMetrics{
			Name: "container1",
			Usage: v1.ResourceList{
				v1.ResourceCPU:    *resource.NewScaledQuantity(1*CoreSecond, -9),
				v1.ResourceMemory: *resource.NewMilliQuantity(5*MiByte, resource.BinarySI),
			},
		}))
		By("return empty for not stored pod2")
		checkPodResponseEmpty(s, types.NamespacedName{Namespace: "ns1", Name: "pod2"})

		By("storing third batch without metrics")
		s.Store(podMetricsBatch())

		By("return empty result for pod1")
		checkPodResponseEmpty(s, podRef)

	})
	It("returns timestamp of earliest container of pod", func() {
		s := NewStorage()
		containerStart := time.Now()
		podRef := types.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("store first batch")
		s.Store(podMetricsBatch(podMetrics(podRef,
			ContainerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(10*time.Second), 1*CoreSecond, 4*MiByte)},
			ContainerMetricsPoint{"container2", newMetricsPoint(containerStart, containerStart.Add(15*time.Second), 2*CoreSecond, 5*MiByte)},
		)))

		By("store second batch")
		s.Store(podMetricsBatch(podMetrics(podRef,
			ContainerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(20*time.Second), 6*CoreSecond, 6*MiByte)},
			ContainerMetricsPoint{"container2", newMetricsPoint(containerStart, containerStart.Add(25*time.Second), 7*CoreSecond, 7*MiByte)},
		)))

		By("returning correct metric values")
		Expect(s.Ready()).NotTo(BeTrue())
		ts, _, err := s.GetPodMetrics(podRef)
		Expect(err).NotTo(HaveOccurred())
		Expect(ts).To(HaveLen(1))
		Expect(ts[0]).To(Equal(api.TimeInfo{
			Timestamp: containerStart.Add(20 * time.Second),
			Window:    10 * time.Second,
		}))
	})
	It("should handle duplicate pod", func() {
		s := NewStorage()
		containerStart := time.Now()
		podRef := types.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("store first batch")
		s.Store(podMetricsBatch(
			podMetrics(podRef, ContainerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(10*time.Second), 1*CoreSecond, 4*MiByte)}),
			podMetrics(podRef, ContainerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(10*time.Second), 1*CoreSecond, 4*MiByte)}),
		))

		By("store second batch")
		s.Store(podMetricsBatch(
			podMetrics(podRef, ContainerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(15*time.Second), 6*CoreSecond, 5*MiByte)}),
			podMetrics(podRef, ContainerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(15*time.Second), 6*CoreSecond, 5*MiByte)}),
		))

		By("returning correct metric values")
		Expect(s.Ready()).NotTo(BeTrue())
		ts, ms, err := s.GetPodMetrics(podRef)
		Expect(err).NotTo(HaveOccurred())
		Expect(ts).To(HaveLen(1))
		Expect(ts[0]).To(Equal(api.TimeInfo{
			Timestamp: containerStart.Add(15 * time.Second),
			Window:    5 * time.Second,
		}))
		Expect(ms).To(HaveLen(1))
		Expect(ms[0]).To(HaveLen(1))
		Expect(ms[0][0]).To(Equal(metrics.ContainerMetrics{
			Name: "container1",
			Usage: v1.ResourceList{
				v1.ResourceCPU:    *resource.NewScaledQuantity(1*CoreSecond, -9),
				v1.ResourceMemory: *resource.NewMilliQuantity(5*MiByte, resource.BinarySI),
			},
		}))
	})
	It("handle repeated pod metric point", func() {
		s := NewStorage()
		containerStart := time.Now()
		podRef := types.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("storing first batch with pod1 metrics")
		batch := podMetricsBatch(podMetrics(podRef, ContainerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(10*time.Second), 1*CoreSecond, 4*MiByte)}))
		s.Store(batch)
		By("storing second batch with exactly same metric")
		s.Store(batch)

		By("return empty results for pod1")
		checkPodResponseEmpty(s, podRef)
	})
	It("exposes correct pod metrics", func() {
		pointsStored.Create(nil)
		pointsStored.Reset()
		s := NewStorage()
		containerStart := time.Now()
		podRef := types.NamespacedName{Name: "pod1", Namespace: "ns1"}

		err := testutil.CollectAndCompare(pointsStored, strings.NewReader(`
		`), "metrics_server_storage_points")
		Expect(err).NotTo(HaveOccurred())

		By("store first batch")
		s.Store(podMetricsBatch(podMetrics(podRef,
			ContainerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(10*time.Second), 1*CoreSecond, 4*MiByte)},
			ContainerMetricsPoint{"container2", newMetricsPoint(containerStart, containerStart.Add(15*time.Second), 2*CoreSecond, 5*MiByte)},
		)))

		err = testutil.CollectAndCompare(pointsStored, strings.NewReader(`
		# HELP metrics_server_storage_points [ALPHA] Number of metrics points stored.
		# TYPE metrics_server_storage_points gauge
		metrics_server_storage_points{type="container"} 0
		metrics_server_storage_points{type="node"} 0
		`), "metrics_server_storage_points")
		Expect(err).NotTo(HaveOccurred())

		By("store second batch")
		s.Store(podMetricsBatch(podMetrics(podRef,
			ContainerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(20*time.Second), 6*CoreSecond, 6*MiByte)},
			ContainerMetricsPoint{"container2", newMetricsPoint(containerStart, containerStart.Add(25*time.Second), 7*CoreSecond, 7*MiByte)},
		)))

		err = testutil.CollectAndCompare(pointsStored, strings.NewReader(`
		# HELP metrics_server_storage_points [ALPHA] Number of metrics points stored.
		# TYPE metrics_server_storage_points gauge
		metrics_server_storage_points{type="container"} 2
		metrics_server_storage_points{type="node"} 0
		`), "metrics_server_storage_points")
		Expect(err).NotTo(HaveOccurred())
	})
	It("should detect container restart and skip metric", func() {
		s := NewStorage()
		containerStart := time.Now()
		podRef := types.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("storing first batch with pod1 metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, ContainerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(10*time.Second), 1*CoreSecond, 4*MiByte)})))

		By("storing second batch with pod1 start time after previous batch")
		s.Store(podMetricsBatch(podMetrics(podRef, ContainerMetricsPoint{"container1", newMetricsPoint(containerStart.Add(15*time.Second), containerStart.Add(20*time.Second), 5*CoreSecond, 5*MiByte)})))

		By("return empty result for restarted pod1")
		checkPodResponseEmpty(s, podRef)
	})
})

func checkPodResponseEmpty(s *storage, pods ...types.NamespacedName) {
	ts, ms, err := s.GetPodMetrics(pods...)
	Expect(err).NotTo(HaveOccurred())
	Expect(ts).To(HaveLen(len(pods)))
	Expect(ms).To(HaveLen(len(pods)))
	for i := range pods {
		Expect(ts[i].Timestamp.IsZero()).To(BeTrue())
		Expect(ms[i]).To(BeNil())
	}
}

func podMetricsBatch(pods ...PodMetricsPoint) *MetricsBatch {
	return &MetricsBatch{
		Pods: pods,
	}
}

func podMetrics(pod types.NamespacedName, cs ...ContainerMetricsPoint) PodMetricsPoint {
	return PodMetricsPoint{
		Namespace:  pod.Namespace,
		Name:       pod.Name,
		Containers: cs,
	}
}
