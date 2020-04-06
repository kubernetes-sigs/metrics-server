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
	"testing"
	"time"

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

type fakePodMetricsGetter struct{}

var _ PodMetricsGetter = (*fakePodMetricsGetter)(nil)

func (mp fakePodMetricsGetter) GetContainerMetrics(pods ...apitypes.NamespacedName) ([]TimeInfo, [][]metrics.ContainerMetrics, error) {
	return []TimeInfo{
			{Timestamp: time.Now(), Window: 1000}, {Timestamp: time.Now(), Window: 2000}, {Timestamp: time.Now(), Window: 3000},
		}, [][]metrics.ContainerMetrics{
			{
				{Name: "metric1", Usage: v1.ResourceList{v1.ResourceCPU: resource.MustParse("10m")}},
				{Name: "metric1-b", Usage: v1.ResourceList{v1.ResourceMemory: resource.MustParse("5Mi")}},
			},
			{{Name: "metric2", Usage: v1.ResourceList{v1.ResourceCPU: resource.MustParse("20m"), v1.ResourceMemory: resource.MustParse("15Mi")}}},
			{{Name: "metric3", Usage: v1.ResourceList{v1.ResourceCPU: resource.MustParse("20m"), v1.ResourceMemory: resource.MustParse("25Mi")}}},
		}, nil
}

func NewPodTestStorage(resp interface{}, err error) *podMetrics {
	return &podMetrics{
		podLister: fakePodLister{
			resp: resp,
			err:  err,
		},
		metrics: fakePodMetricsGetter{},
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

	if len(res.Items) != 3 ||
		res.Items[0].Containers[0].Name != "metric1" ||
		res.Items[1].Containers[0].Name != "metric2" ||
		res.Items[2].Containers[0].Name != "metric3" {
		t.Errorf("Got unexpected object: %+v", got)
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
		res.Rows[1].Cells[0] != "pod3" ||
		res.Rows[1].Cells[1] != "20m" ||
		res.Rows[1].Cells[2] != "15Mi" ||
		res.Rows[1].Cells[3] != "2µs" ||
		res.Rows[2].Cells[0] != "pod2" ||
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

func TestPodList_PodNotRunning(t *testing.T) {
	// setup
	pods := createTestPods()
	pods[1].Status.Phase = v1.PodPending

	r := NewPodTestStorage(pods, nil)

	// execute
	got, err := r.List(genericapirequest.NewContext(), nil)

	// assert
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	res := got.(*metrics.PodMetricsList)

	if len(res.Items) != 2 ||
		res.Items[0].Name != "pod1" ||
		res.Items[1].Name != "pod3" {
		t.Errorf("Got unexpected object: %+v", got)
	}
}

func createTestPods() []*v1.Pod {
	pod1 := &v1.Pod{}
	pod1.Namespace = "other"
	pod1.Name = "pod1"
	pod1.Status.Phase = v1.PodRunning
	pod2 := &v1.Pod{}
	pod2.Namespace = "testValue"
	pod2.Name = "pod2"
	pod2.Status.Phase = v1.PodRunning
	pod3 := &v1.Pod{}
	pod3.Namespace = "other"
	pod3.Name = "pod3"
	pod3.Status.Phase = v1.PodRunning
	return []*v1.Pod{pod1, pod2, pod3}
}
