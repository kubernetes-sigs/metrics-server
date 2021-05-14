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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"k8s.io/component-base/metrics/testutil"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	fields "k8s.io/apimachinery/pkg/fields"
	labels "k8s.io/apimachinery/pkg/labels"

	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/diff"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/metrics/pkg/apis/metrics"
)

// fakes both PodLister and PodNamespaceLister at once
type fakePodLister struct {
	resp interface{}
	err  error
}

func (pl fakePodLister) List(selector labels.Selector) (ret []*v1.Pod, err error) {
	return pl.resp.([]*v1.Pod), pl.err
}
func (pl fakePodLister) Get(name string) (*v1.Pod, error) {
	return pl.resp.(*v1.Pod), pl.err
}
func (pl fakePodLister) Pods(namespace string) listerv1.PodNamespaceLister {
	return pl
}

type fakePodMetricsGetter struct {
	time    []TimeInfo
	metrics [][]metrics.ContainerMetrics
}

var _ PodMetricsGetter = (*fakePodMetricsGetter)(nil)

func (mp fakePodMetricsGetter) GetPodMetrics(pods ...apitypes.NamespacedName) ([]TimeInfo, [][]metrics.ContainerMetrics, error) {
	return mp.time, mp.metrics, nil
}

func NewPodTestStorage(resp interface{}, err error) *podMetrics {
	return &podMetrics{
		podLister: fakePodLister{
			resp: resp,
			err:  err,
		},
		metrics: fakePodMetricsGetter{
			time: []TimeInfo{
				{Timestamp: myClock.Now(), Window: 1000},
				{Timestamp: myClock.Now(), Window: 2000},
				{Timestamp: myClock.Now(), Window: 3000},
			},
			metrics: [][]metrics.ContainerMetrics{
				{
					{Name: "metric1", Usage: v1.ResourceList{v1.ResourceCPU: resource.MustParse("10m")}},
					{Name: "metric1-b", Usage: v1.ResourceList{v1.ResourceMemory: resource.MustParse("5Mi")}},
				},
				{{Name: "metric2", Usage: v1.ResourceList{v1.ResourceCPU: resource.MustParse("20m"), v1.ResourceMemory: resource.MustParse("15Mi")}}},
				{{Name: "metric3", Usage: v1.ResourceList{v1.ResourceCPU: resource.MustParse("20m"), v1.ResourceMemory: resource.MustParse("25Mi")}}},
			},
		},
	}
}

func TestPodList_NoError(t *testing.T) {
	// setup
	r := NewPodTestStorage(createTestPods(), nil)

	// execute
	got, err := r.List(genericapirequest.NewContext(), nil)

	// assert
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	res := got.(*metrics.PodMetricsList)

	if len(res.Items) != 3 {
		t.Fatalf("len(res.Items) != 3, got: %+v", res.Items)
	}
	testPod(t, res.Items[0], "pod1", "other", "metric1", map[string]string{"labelKey": "labelValue"})
	testPod(t, res.Items[1], "pod2", "other", "metric2", map[string]string{"otherKey": "labelValue"})
	testPod(t, res.Items[2], "pod3", "testValue", "metric3", map[string]string{"labelKey": "otherValue"})
}

func testPod(t *testing.T, got metrics.PodMetrics, wantName, wantNamespace, wantContainer string, wantLabels map[string]string) {
	t.Helper()
	if got.Name != wantName {
		t.Errorf(`Name != "%s", got: %+v`, wantName, got.Name)
	}
	if got.Namespace != wantNamespace {
		t.Errorf(`Namespace != "%s", got: %+v`, wantNamespace, got.Namespace)
	}
	if got.Containers[0].Name != wantContainer {
		t.Errorf(`Containers[0].Name != "%s", got: %+v`, wantContainer, got.Containers[0].Name)
	}
	if diff := cmp.Diff(got.Labels, wantLabels); diff != "" {
		t.Errorf(`Labels != %+v, diff: %s`, wantLabels, diff)
	}
}

func TestPodList_ConvertToTable(t *testing.T) {
	// setup
	r := NewPodTestStorage(createTestPods(), nil)

	// execute
	got, err := r.List(genericapirequest.NewContext(), nil)

	// assert
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	res, err := r.ConvertToTable(genericapirequest.NewContext(), got, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(res.Rows) != 3 ||
		res.ColumnDefinitions[1].Name != "cpu" || res.ColumnDefinitions[2].Name != "memory" || res.ColumnDefinitions[3].Name != "Window" ||
		res.Rows[0].Cells[0] != "pod1" ||
		res.Rows[0].Cells[1] != "10m" ||
		res.Rows[0].Cells[2] != "5Mi" ||
		res.Rows[0].Cells[3] != "1µs" ||
		res.Rows[1].Cells[0] != "pod2" ||
		res.Rows[1].Cells[1] != "20m" ||
		res.Rows[1].Cells[2] != "15Mi" ||
		res.Rows[1].Cells[3] != "2µs" ||
		res.Rows[2].Cells[0] != "pod3" ||
		res.Rows[2].Cells[1] != "20m" ||
		res.Rows[2].Cells[2] != "25Mi" ||
		res.Rows[2].Cells[3] != "3µs" {
		t.Errorf("Got unexpected object: %+v", res)
	}
}

func TestPodList_EmptyResponse(t *testing.T) {
	// setup
	r := NewPodTestStorage([]*v1.Pod{}, nil)

	// execute
	got, err := r.List(genericapirequest.NewContext(), nil)

	// assert
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expect := &metrics.PodMetricsList{Items: []metrics.PodMetrics{}}
	if e, a := expect, got; !reflect.DeepEqual(e, a) {
		t.Errorf("Got unexpected object: %+v", diff.ObjectDiff(e, a))
	}
}

func TestPodList_WithFieldSelectors(t *testing.T) {
	// setup
	r := NewPodTestStorage(createTestPods(), nil)

	opts := &metainternalversion.ListOptions{
		FieldSelector: fields.SelectorFromSet(map[string]string{
			"metadata.namespace": "testValue",
		}),
	}

	// execute
	got, err := r.List(genericapirequest.NewContext(), opts)

	// assert
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	res := got.(*metrics.PodMetricsList)

	if len(res.Items) != 1 || res.Items[0].Containers[0].Name != "metric1" {
		t.Errorf("Got unexpected object: %+v", got)
	}
}

func TestPodList_Monitoring(t *testing.T) {
	c := &fakeClock{}
	myClock = c

	metricFreshness.Create(nil)
	metricFreshness.Reset()

	r := NewPodTestStorage(createTestPods(), nil)
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

func createTestPods() []*v1.Pod {
	pod1 := &v1.Pod{}
	pod1.Namespace = "other"
	pod1.Name = "pod1"
	pod1.Status.Phase = v1.PodRunning
	pod1.Labels = map[string]string{
		"labelKey": "labelValue",
	}
	pod2 := &v1.Pod{}
	pod2.Namespace = "other"
	pod2.Name = "pod2"
	pod2.Status.Phase = v1.PodRunning
	pod2.Labels = map[string]string{
		"otherKey": "labelValue",
	}
	pod3 := &v1.Pod{}
	pod3.Namespace = "testValue"
	pod3.Name = "pod3"
	pod3.Status.Phase = v1.PodRunning
	pod3.Labels = map[string]string{
		"labelKey": "otherValue",
	}
	return []*v1.Pod{pod1, pod2, pod3}
}
