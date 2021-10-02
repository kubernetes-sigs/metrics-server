/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/tools/cache"
	"k8s.io/component-base/metrics/testutil"
	"k8s.io/metrics/pkg/apis/metrics"
)

func TestPodList(t *testing.T) {
	tcs := []struct {
		name        string
		listOptions *metainternalversion.ListOptions
		listerError error
		wantPods    []apitypes.NamespacedName
		wantError   bool
	}{
		{
			name:     "Normal",
			wantPods: []apitypes.NamespacedName{{Name: "pod1", Namespace: "other"}, {Name: "pod2", Namespace: "other"}, {Name: "pod3", Namespace: "testValue"}},
		},
		{
			name: "Empty response",
			listOptions: &metainternalversion.ListOptions{
				FieldSelector: fields.SelectorFromSet(map[string]string{
					"metadata.namespace": "unknown",
				}),
			},
		},
		{
			name: "With FieldOptions",
			listOptions: &metainternalversion.ListOptions{
				FieldSelector: fields.SelectorFromSet(map[string]string{
					"metadata.namespace": "testValue",
				}),
			},
			wantPods: []apitypes.NamespacedName{{Name: "pod3", Namespace: "testValue"}},
		},
		{
			name: "With Label selectors",
			listOptions: &metainternalversion.ListOptions{
				LabelSelector: labels.SelectorFromSet(map[string]string{
					"labelKey": "labelValue",
				}),
			},
			wantPods: []apitypes.NamespacedName{{Name: "pod1", Namespace: "other"}},
		},
		{
			name: "With both fields and label selectors",
			listOptions: &metainternalversion.ListOptions{
				FieldSelector: fields.SelectorFromSet(map[string]string{
					"metadata.name": "pod3",
				}),
				LabelSelector: labels.SelectorFromSet(map[string]string{
					"labelKey": "otherValue",
				}),
			},
			wantPods: []apitypes.NamespacedName{{Name: "pod3", Namespace: "testValue"}},
		},
		{
			name:        "Lister error",
			listerError: fmt.Errorf("lister error"),
			wantPods:    []apitypes.NamespacedName{},
			wantError:   true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// setup
			r := NewPodTestStorage(tc.listerError)

			// execute
			got, err := r.List(genericapirequest.NewContext(), tc.listOptions)

			// assert
			if (err != nil) != tc.wantError {
				t.Fatalf("Unexpected error: %v", err)
			}
			res := got.(*metrics.PodMetricsList)

			if len(res.Items) != len(tc.wantPods) {
				t.Fatalf("len(res.Items) != %d, got: %d", len(tc.wantPods), len(res.Items))
			}
			for i := range res.Items {
				testPod(t, res.Items[i], tc.wantPods[i])
			}
		})
	}
}

func TestPodGet(t *testing.T) {
	tcs := []struct {
		name        string
		pods        *corev1.Pod
		get         apitypes.NamespacedName
		listerError error
		wantPod     apitypes.NamespacedName
		wantError   bool
	}{
		{
			name:    "Normal",
			pods:    createTestPods()[0],
			get:     apitypes.NamespacedName{Name: "pod1", Namespace: "other"},
			wantPod: apitypes.NamespacedName{Name: "pod1", Namespace: "other"},
		},
		{
			name:        "Lister error",
			get:         apitypes.NamespacedName{Name: "pod1", Namespace: "other"},
			listerError: fmt.Errorf("lister error"),
			wantError:   true,
		},
		{
			name:      "Pod without metrics",
			get:       apitypes.NamespacedName{Name: "pod4", Namespace: "testValue"},
			wantError: true,
		},
		{
			name:      "Pod doesn't exist",
			get:       apitypes.NamespacedName{Name: "pod5", Namespace: "other"},
			wantError: true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// setup
			r := NewPodTestStorage(tc.listerError)

			// execute
			got, err := r.Get(genericapirequest.NewContext(), tc.get.Name, nil)

			// assert
			if (err != nil) != tc.wantError {
				t.Fatalf("Unexpected error: %v", err)
			}
			if tc.wantError {
				return
			}
			res := got.(*metrics.PodMetrics)
			testPod(t, *res, tc.wantPod)
		})
	}
}

func TestPodList_Monitoring(t *testing.T) {
	c := &fakeClock{}
	myClock = c

	metricFreshness.Create(nil)
	metricFreshness.Reset()

	r := NewPodTestStorage(nil)
	c.now = c.now.Add(10 * time.Second)
	_, err := r.List(genericapirequest.NewContext(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = testutil.CollectAndCompare(metricFreshness, strings.NewReader(`
	# HELP metrics_server_api_metric_freshness_seconds [ALPHA] Freshness of metrics exported
	# TYPE metrics_server_api_metric_freshness_seconds histogram
	metrics_server_api_metric_freshness_seconds_bucket{le="1"} 0
	metrics_server_api_metric_freshness_seconds_bucket{le="1.364"} 0
	metrics_server_api_metric_freshness_seconds_bucket{le="1.8604960000000004"} 0
	metrics_server_api_metric_freshness_seconds_bucket{le="2.5377165440000007"} 0
	metrics_server_api_metric_freshness_seconds_bucket{le="3.4614453660160014"} 0
	metrics_server_api_metric_freshness_seconds_bucket{le="4.721411479245826"} 0
	metrics_server_api_metric_freshness_seconds_bucket{le="6.440005257691307"} 0
	metrics_server_api_metric_freshness_seconds_bucket{le="8.784167171490942"} 0
	metrics_server_api_metric_freshness_seconds_bucket{le="11.981604021913647"} 3
	metrics_server_api_metric_freshness_seconds_bucket{le="16.342907885890217"} 3
	metrics_server_api_metric_freshness_seconds_bucket{le="22.291726356354257"} 3
	metrics_server_api_metric_freshness_seconds_bucket{le="30.405914750067208"} 3
	metrics_server_api_metric_freshness_seconds_bucket{le="41.47366771909167"} 3
	metrics_server_api_metric_freshness_seconds_bucket{le="56.57008276884105"} 3
	metrics_server_api_metric_freshness_seconds_bucket{le="77.16159289669919"} 3
	metrics_server_api_metric_freshness_seconds_bucket{le="105.2484127110977"} 3
	metrics_server_api_metric_freshness_seconds_bucket{le="143.55883493793726"} 3
	metrics_server_api_metric_freshness_seconds_bucket{le="195.81425085534644"} 3
	metrics_server_api_metric_freshness_seconds_bucket{le="267.09063816669254"} 3
	metrics_server_api_metric_freshness_seconds_bucket{le="364.31163045936864"} 3
	metrics_server_api_metric_freshness_seconds_bucket{le="+Inf"} 3
	metrics_server_api_metric_freshness_seconds_sum 30
	metrics_server_api_metric_freshness_seconds_count 3
	`), "metrics_server_api_metric_freshness_seconds")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// fakes both PodLister and PodNamespaceLister at once
type fakePodLister struct {
	data []*corev1.Pod
	err  error
}

func (pl fakePodLister) List(selector labels.Selector) (ret []runtime.Object, err error) {
	if pl.err != nil {
		return nil, pl.err
	}
	res := []runtime.Object{}
	for _, pod := range pl.data {
		if selector.Matches(labels.Set(pod.Labels)) {
			res = append(res, &metav1.PartialObjectMetadata{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pod.Name,
					Namespace: pod.Namespace,
					Labels:    pod.Labels,
				},
			})
		}
	}
	return res, nil
}
func (pl fakePodLister) Get(name string) (runtime.Object, error) {
	if pl.err != nil {
		return nil, pl.err
	}
	for _, pod := range pl.data {
		if pod.Name == name {
			return &metav1.PartialObjectMetadata{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pod.Name,
					Namespace: pod.Namespace,
					Labels:    pod.Labels,
				},
			}, nil
		}
	}
	return nil, nil
}
func (pl fakePodLister) ByNamespace(namespace string) cache.GenericNamespaceLister {
	return pl
}

type fakePodMetricsGetter struct {
	now time.Time
}

var _ PodMetricsGetter = (*fakePodMetricsGetter)(nil)

func (mp fakePodMetricsGetter) GetPodMetrics(pods ...*metav1.PartialObjectMetadata) ([]metrics.PodMetrics, error) {
	ms := make([]metrics.PodMetrics, 0, len(pods))
	for _, pod := range pods {
		switch {
		case pod.Name == "pod1" && pod.Namespace == "other":
			ms = append(ms, metrics.PodMetrics{
				ObjectMeta: metav1.ObjectMeta{Name: pod.Name, Namespace: pod.Namespace, Labels: pod.Labels},
				Timestamp:  metav1.Time{Time: mp.now},
				Window:     metav1.Duration{Duration: 1000},
				Containers: []metrics.ContainerMetrics{
					{Name: "metric1", Usage: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10m")}},
					{Name: "metric1-b", Usage: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("5Mi")}},
				},
			})
		case pod.Name == "pod2" && pod.Namespace == "other":
			ms = append(ms, metrics.PodMetrics{
				ObjectMeta: metav1.ObjectMeta{Name: pod.Name, Namespace: pod.Namespace, Labels: pod.Labels},
				Timestamp:  metav1.Time{Time: mp.now},
				Window:     metav1.Duration{Duration: 2000},
				Containers: []metrics.ContainerMetrics{
					{Name: "metric2", Usage: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("20m"), corev1.ResourceMemory: resource.MustParse("15Mi")}},
				},
			})
		case pod.Name == "pod3" && pod.Namespace == "testValue":
			ms = append(ms, metrics.PodMetrics{
				ObjectMeta: metav1.ObjectMeta{Name: pod.Name, Namespace: pod.Namespace, Labels: pod.Labels},
				Timestamp:  metav1.Time{Time: mp.now},
				Window:     metav1.Duration{Duration: 3000},
				Containers: []metrics.ContainerMetrics{
					{Name: "metric3", Usage: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("20m"), corev1.ResourceMemory: resource.MustParse("25Mi")}},
				},
			})
		}
	}
	return ms, nil
}

func NewPodTestStorage(listerError error) *podMetrics {
	return &podMetrics{
		podLister: fakePodLister{data: createTestPods(), err: listerError},
		metrics:   fakePodMetricsGetter{now: myClock.Now()},
	}
}

func testPod(t *testing.T, got metrics.PodMetrics, want apitypes.NamespacedName) {
	t.Helper()
	if got.Name != want.Name {
		t.Errorf(`Name != "%s", got: %+v`, want.Name, got.Name)
	}
	if got.Namespace != want.Namespace {
		t.Errorf(`Namespace != "%s", got: %+v`, want.Namespace, got.Namespace)
	}
	wantLabels := podLabels(want.Name, want.Namespace)
	if diff := cmp.Diff(got.Labels, wantLabels); diff != "" {
		t.Errorf(`Labels != %+v, diff: %s`, wantLabels, diff)
	}
}

func createTestPods() []*corev1.Pod {
	pod1 := &corev1.Pod{}
	pod1.Namespace = "other"
	pod1.Name = "pod1"
	pod1.Status.Phase = corev1.PodRunning
	pod1.Labels = podLabels(pod1.Name, pod1.Namespace)
	pod2 := &corev1.Pod{}
	pod2.Namespace = "other"
	pod2.Name = "pod2"
	pod2.Status.Phase = corev1.PodRunning
	pod2.Labels = podLabels(pod2.Name, pod2.Namespace)
	pod3 := &corev1.Pod{}
	pod3.Namespace = "testValue"
	pod3.Name = "pod3"
	pod3.Status.Phase = corev1.PodRunning
	pod3.Labels = podLabels(pod3.Name, pod3.Namespace)
	pod4 := &corev1.Pod{}
	pod4.Namespace = "other"
	pod4.Name = "pod4"
	pod4.Status.Phase = corev1.PodRunning
	pod4.Labels = podLabels(pod4.Name, pod4.Namespace)
	return []*corev1.Pod{pod1, pod2, pod3, pod4}
}

func podLabels(name, namespace string) map[string]string {
	var labels map[string]string
	switch {
	case name == "pod1" && namespace == "other":
		labels = map[string]string{
			"labelKey": "labelValue",
		}
	case name == "pod2" && namespace == "other":
		labels = map[string]string{
			"otherKey": "labelValue",
		}
	case name == "pod3" && namespace == "testValue":
		labels = map[string]string{
			"labelKey": "otherValue",
		}
	case name == "pod4" && namespace == "testValue":
		labels = map[string]string{
			"otherKey": "otherValue",
		}
	}
	return labels
}
