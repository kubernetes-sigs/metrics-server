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
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/component-base/metrics/testutil"
	"k8s.io/metrics/pkg/apis/metrics"
)

var _ = Describe("Pod storage", func() {
	It("provides pod metrics from stored batches", func() {
		s := NewStorage(60 * time.Second)
		containerStart := time.Now()
		podRef := apitypes.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("storing first batch with pod1 metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(120*time.Second), 1*CoreSecond, 4*MiByte)})))

		By("waiting for second batch before serving metrics")
		Expect(s.Ready()).NotTo(BeTrue())
		checkPodResponseEmpty(s, podRef)

		By("storing second batch with pod1 metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(125*time.Second), 6*CoreSecond, 5*MiByte)})))

		By("returning metric for pod1")
		Expect(s.Ready()).To(BeTrue())
		ms, err := s.GetPodMetrics(&metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: podRef.Name, Namespace: podRef.Namespace}})
		Expect(err).NotTo(HaveOccurred())
		Expect(ms).To(HaveLen(1))
		Expect(ms[0].Timestamp.Time).Should(BeEquivalentTo(containerStart.Add(125 * time.Second)))
		Expect(ms[0].Window.Duration).Should(BeEquivalentTo(5 * time.Second))
		Expect(ms[0].Containers).Should(BeEquivalentTo([]metrics.ContainerMetrics{{
			Name: "container1",
			Usage: corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewScaledQuantity(1*CoreSecond, -9),
				corev1.ResourceMemory: *resource.NewQuantity(5*MiByte, resource.BinarySI),
			},
		}}))
		By("return empty for not stored pod2")
		checkPodResponseEmpty(s, apitypes.NamespacedName{Namespace: "ns1", Name: "pod2"})

		By("storing third batch without metrics")
		s.Store(podMetricsBatch())

		By("return empty result for pod1")
		checkPodResponseEmpty(s, podRef)

	})
	It("returns timestamp of earliest container of pod", func() {
		s := NewStorage(60 * time.Second)
		containerStart := time.Now()
		podRef := apitypes.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("store first batch")
		s.Store(podMetricsBatch(podMetrics(podRef,
			containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(110*time.Second), 1*CoreSecond, 4*MiByte)},
			containerMetricsPoint{"container2", newMetricsPoint(containerStart, containerStart.Add(115*time.Second), 2*CoreSecond, 5*MiByte)},
		)))

		By("store second batch")
		s.Store(podMetricsBatch(podMetrics(podRef,
			containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(120*time.Second), 6*CoreSecond, 6*MiByte)},
			containerMetricsPoint{"container2", newMetricsPoint(containerStart, containerStart.Add(125*time.Second), 7*CoreSecond, 7*MiByte)},
		)))

		By("returning correct metric values")
		Expect(s.Ready()).To(BeTrue())
		ms, err := s.GetPodMetrics(&metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: podRef.Name, Namespace: podRef.Namespace}})
		Expect(err).NotTo(HaveOccurred())
		Expect(ms).Should(HaveLen(1))
		Expect(ms[0].Timestamp.Time).Should(BeEquivalentTo(containerStart.Add(120 * time.Second)))
		Expect(ms[0].Window.Duration).Should(BeEquivalentTo(10 * time.Second))
	})
	It("handle repeated pod metric point", func() {
		s := NewStorage(60 * time.Second)
		containerStart := time.Now()
		podRef := apitypes.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("storing first batch with pod1 metrics")
		batch := podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(110*time.Second), 1*CoreSecond, 4*MiByte)}))
		s.Store(batch)
		By("storing second batch with exactly same metric")
		s.Store(batch)

		By("return empty results for pod1")
		Expect(s.Ready()).NotTo(BeTrue())
		checkPodResponseEmpty(s, podRef)
	})
	It("exposes correct pod metrics", func() {
		pointsStored.Create(nil)
		pointsStored.Reset()
		s := NewStorage(60 * time.Second)
		containerStart := time.Now()
		podRef := apitypes.NamespacedName{Name: "pod1", Namespace: "ns1"}

		err := testutil.CollectAndCompare(pointsStored, strings.NewReader(`
		`), "metrics_server_storage_points")
		Expect(err).NotTo(HaveOccurred())

		By("store first batch")
		s.Store(podMetricsBatch(podMetrics(podRef,
			containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(110*time.Second), 1*CoreSecond, 4*MiByte)},
			containerMetricsPoint{"container2", newMetricsPoint(containerStart, containerStart.Add(115*time.Second), 2*CoreSecond, 5*MiByte)},
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
			containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(120*time.Second), 6*CoreSecond, 6*MiByte)},
			containerMetricsPoint{"container2", newMetricsPoint(containerStart, containerStart.Add(125*time.Second), 7*CoreSecond, 7*MiByte)},
		)))

		err = testutil.CollectAndCompare(pointsStored, strings.NewReader(`
		# HELP metrics_server_storage_points [ALPHA] Number of metrics points stored.
		# TYPE metrics_server_storage_points gauge
		metrics_server_storage_points{type="container"} 2
		metrics_server_storage_points{type="node"} 0
		`), "metrics_server_storage_points")
		Expect(err).NotTo(HaveOccurred())
	})
	It("should detect container restart and return results based on window from start time", func() {
		s := NewStorage(60 * time.Second)
		containerStart := time.Now()
		podRef := apitypes.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("storing first batch with pod1 metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(10*time.Second), 1*CoreSecond, 4*MiByte)})))

		By("storing second batch with pod1 start time after previous batch")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart.Add(15*time.Second), containerStart.Add(25*time.Second), 5*CoreSecond, 5*MiByte)})))

		By("return results based on window from start time")
		ms, err := s.GetPodMetrics(&metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: podRef.Name, Namespace: podRef.Namespace}})
		Expect(err).NotTo(HaveOccurred())
		Expect(ms).To(HaveLen(1))
		Expect(ms[0].Timestamp.Time).Should(BeEquivalentTo(containerStart.Add(25 * time.Second)))
		Expect(ms[0].Window.Duration).Should(BeEquivalentTo(10 * time.Second))
		Expect(ms[0].Containers).Should(BeEquivalentTo([]metrics.ContainerMetrics{{
			Name: "container1",
			Usage: corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewScaledQuantity(500000000, -9),
				corev1.ResourceMemory: *resource.NewQuantity(5*MiByte, resource.BinarySI),
			},
		}}))
	})
	It("should return pod empty metrics if decreased data point reported", func() {
		s := NewStorage(60 * time.Second)
		containerStart := time.Now()
		podRef := apitypes.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("storing previous metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(110*time.Second), 20*CoreSecond, 4*MiByte)})))

		By("storing CPU usage decreased last metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(120*time.Second), 10*CoreSecond, 4*MiByte)})))

		By("should get empty metrics when cpu metrics decrease")
		ms, err := s.GetPodMetrics(&metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: podRef.Name, Namespace: podRef.Namespace}})
		Expect(err).NotTo(HaveOccurred())
		Expect(ms).To(HaveLen(1))
		Expect(ms[0].Containers).To(HaveLen(0))
	})
	It("should handle pod metrics older than prev", func() {
		s := NewStorage(60 * time.Second)
		containerStart := time.Now()
		podRef := apitypes.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("storing previous metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(1115*time.Second), 20*CoreSecond, 4*MiByte)})))

		By("storing last metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(1125*time.Second), 40*CoreSecond, 4*MiByte)})))

		By("Storing new metrics older than previous")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(115*time.Second), 10*CoreSecond, 4*MiByte)})))

		By("should get empty metrics after stored older metrics than previous")
		checkPodResponseEmpty(s, podRef)
	})
	It("should handle pod metrics prev.ts < newNode.ts < last.ts", func() {
		s := NewStorage(60 * time.Second)
		containerStart := time.Now()
		podRef := apitypes.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("storing previous metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(115*time.Second), 10*CoreSecond, 4*MiByte)})))

		By("storing last metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(125*time.Second), 50*CoreSecond, 5*MiByte)})))

		By("Storing new metrics prev.ts < node.ts < last.ts")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(120*time.Second), 35*CoreSecond, 5*MiByte)})))

		By("should get non-empty metrics after stored older metrics than previous")
		ms, err := s.GetPodMetrics(&metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: podRef.Name, Namespace: podRef.Namespace}})
		Expect(err).NotTo(HaveOccurred())
		Expect(ms).To(HaveLen(1))
		Expect(ms[0].Timestamp.Time).Should(BeEquivalentTo(containerStart.Add(120 * time.Second)))
		Expect(ms[0].Window.Duration).Should(BeEquivalentTo(5 * time.Second))
		Expect(ms[0].Containers).Should(BeEquivalentTo([]metrics.ContainerMetrics{{
			Name: "container1",
			Usage: corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewScaledQuantity(5*CoreSecond, -9),
				corev1.ResourceMemory: *resource.NewQuantity(5*MiByte, resource.BinarySI),
			},
		}}))
	})
	It("should not use start time to return metric in one cycle for long running container", func() {
		s := NewStorage(60 * time.Second)
		containerStart := time.Now()
		podRef := apitypes.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("storing first batch with pod1 metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(120*time.Second), 1*CoreSecond, 4*MiByte)})))
		Expect(s.Ready()).NotTo(BeTrue())
		checkPodResponseEmpty(s, podRef)
	})
	It("should use start time to return metric in one cycle for fresh new container", func() {
		s := NewStorage(60 * time.Second)
		containerStart := time.Now()
		podRef := apitypes.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("storing first batch with pod1 metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(10*time.Second), 10*CoreSecond, 4*MiByte)})))
		Expect(s.Ready()).To(BeTrue())

		ms, err := s.GetPodMetrics(&metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: podRef.Name, Namespace: podRef.Namespace}})
		Expect(err).NotTo(HaveOccurred())
		Expect(ms).To(HaveLen(1))
		Expect(ms[0].Timestamp.Time).Should(BeEquivalentTo(containerStart.Add(10 * time.Second)))
		Expect(ms[0].Window.Duration).Should(BeEquivalentTo(10 * time.Second))
		Expect(ms[0].Containers).Should(BeEquivalentTo([]metrics.ContainerMetrics{{
			Name: "container1",
			Usage: corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewScaledQuantity(1*CoreSecond, -9),
				corev1.ResourceMemory: *resource.NewQuantity(4*MiByte, resource.BinarySI),
			},
		}}))
	})
	It("should get empty metrics in one cycle for fresh new container's start time after timestamp", func() {
		s := NewStorage(60 * time.Second)
		containerStart := time.Now()
		podRef := apitypes.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("storing first batch with pod1 metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(-10*time.Second), 10*CoreSecond, 4*MiByte)})))
		Expect(s.Ready()).NotTo(BeTrue())
		checkPodResponseEmpty(s, podRef)
	})
	It("should get empty metrics in one cycle for fresh new container's time duration less than 10s between start time and timestamp", func() {
		s := NewStorage(60 * time.Second)
		containerStart := time.Now()
		podRef := apitypes.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("storing first batch with pod1 metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(9*time.Second), 10*CoreSecond, 4*MiByte)})))
		Expect(s.Ready()).NotTo(BeTrue())
		checkPodResponseEmpty(s, podRef)
	})

	It("provides pod metrics from stored batches when StartTime is zero", func() {
		s := NewStorage(60 * time.Second)
		containerStart := time.Now()
		podRef := apitypes.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("storing first batch with pod1 metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(time.Time{}, containerStart.Add(120*time.Second), 1*CoreSecond, 4*MiByte)})))

		By("waiting for second batch before serving metrics")
		Expect(s.Ready()).NotTo(BeTrue())
		checkPodResponseEmpty(s, podRef)

		By("storing second batch with pod1 metrics")
		s.Store(podMetricsBatch(podMetrics(podRef, containerMetricsPoint{"container1", newMetricsPoint(time.Time{}, containerStart.Add(125*time.Second), 6*CoreSecond, 5*MiByte)})))

		By("returning metric for pod1")
		Expect(s.Ready()).To(BeTrue())
		ms, err := s.GetPodMetrics(&metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: podRef.Name, Namespace: podRef.Namespace}})
		Expect(err).NotTo(HaveOccurred())
		Expect(ms).To(HaveLen(1))
		Expect(ms[0].Timestamp.Time).Should(BeEquivalentTo(containerStart.Add(125 * time.Second)))
		Expect(ms[0].Window.Duration).Should(BeEquivalentTo(5 * time.Second))
		Expect(ms[0].Containers).Should(BeEquivalentTo([]metrics.ContainerMetrics{{
			Name: "container1",
			Usage: corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewScaledQuantity(1*CoreSecond, -9),
				corev1.ResourceMemory: *resource.NewQuantity(5*MiByte, resource.BinarySI),
			},
		}}))
	})

	It("should get empty metrics if not all containers data points of one pod reported at the first cycle", func() {
		s := NewStorage(60 * time.Second)
		containerStart := time.Now()
		podRef := apitypes.NamespacedName{Name: "pod1", Namespace: "ns1"}

		By("store first batch")
		s.Store(podMetricsBatch(podMetrics(podRef,
			containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(110*time.Second), 1*CoreSecond, 4*MiByte)},
		)))

		By("store second batch")
		s.Store(podMetricsBatch(podMetrics(podRef,
			containerMetricsPoint{"container1", newMetricsPoint(containerStart, containerStart.Add(120*time.Second), 6*CoreSecond, 6*MiByte)},
			containerMetricsPoint{"container2", newMetricsPoint(containerStart, containerStart.Add(120*time.Second), 8*CoreSecond, 5*MiByte)},
		)))

		By("should get empty metrics when not all containers data points of one pod reported")
		ms, err := s.GetPodMetrics(&metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: podRef.Name, Namespace: podRef.Namespace}})
		Expect(err).NotTo(HaveOccurred())
		Expect(ms).To(HaveLen(0))
	})
})

func checkPodResponseEmpty(s *storage, podRef ...apitypes.NamespacedName) {
	pods := []*metav1.PartialObjectMetadata{}
	for _, ref := range podRef {
		pods = append(pods, &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: ref.Name, Namespace: ref.Namespace}})
	}
	ms, err := s.GetPodMetrics(pods...)
	Expect(err).NotTo(HaveOccurred())
	Expect(ms).To(HaveLen(0))
}

func podMetricsBatch(pods ...podMetricsPoint) *MetricsBatch {
	batch := &MetricsBatch{
		Pods: make(map[apitypes.NamespacedName]PodMetricsPoint, len(pods)),
	}
	for _, pod := range pods {
		batch.Pods[pod.NamespacedName] = pod.PodMetricsPoint
	}
	return batch
}

func podMetrics(pod apitypes.NamespacedName, cs ...containerMetricsPoint) podMetricsPoint {
	point := podMetricsPoint{
		NamespacedName:  pod,
		PodMetricsPoint: PodMetricsPoint{Containers: make(map[string]MetricsPoint, len(cs))},
	}
	for _, c := range cs {
		point.Containers[c.Name] = c.MetricsPoint
	}
	return point
}

type podMetricsPoint struct {
	apitypes.NamespacedName
	PodMetricsPoint
}

type containerMetricsPoint struct {
	Name string
	MetricsPoint
}
