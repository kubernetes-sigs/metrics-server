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

package podmetrics

import (
	"reflect"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	fields "k8s.io/apimachinery/pkg/fields"
	labels "k8s.io/apimachinery/pkg/labels"

	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/diff"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/metrics/pkg/apis/metrics"
	"sigs.k8s.io/metrics-server/pkg/provider"
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

type fakeMetricsProvider struct{}

func (mp fakeMetricsProvider) GetContainerMetrics(pods ...apitypes.NamespacedName) ([]provider.TimeInfo, [][]metrics.ContainerMetrics, error) {
	return []provider.TimeInfo{
			{Timestamp: time.Now(), Window: 1000}, {Timestamp: time.Now(), Window: 2000}, {Timestamp: time.Now(), Window: 3000},
		}, [][]metrics.ContainerMetrics{
			{{Name: "metric1"}},
			{{Name: "metric2"}},
			{{Name: "metric3"}},
		}, nil
}

func NewTestStorage(resp interface{}, err error) *MetricStorage {
	return &MetricStorage{
		podLister: fakePodLister{
			resp: resp,
			err:  err,
		},
		prov: fakeMetricsProvider{},
	}
}

func TestList_NoError(t *testing.T) {
	// setup
	r := NewTestStorage(createTestPods(), nil)

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

func TestList_EmptyResponse(t *testing.T) {
	// setup
	r := NewTestStorage([]*v1.Pod{}, nil)

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

func TestList_WithFieldSelectors(t *testing.T) {
	// setup
	r := NewTestStorage(createTestPods(), nil)

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

func TestList_PodNotRunning(t *testing.T) {
	// setup
	pods := createTestPods()
	pods[1].Status.Phase = v1.PodPending

	r := NewTestStorage(pods, nil)

	// execute
	got, err := r.List(genericapirequest.NewContext(), nil)

	// assert
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	res := got.(*metrics.PodMetricsList)

	if len(res.Items) != 2 ||
		res.Items[0].Containers[0].Name != "metric1" ||
		res.Items[1].Containers[0].Name != "metric3" {
		t.Errorf("Got unexpected object: %+v", got)
	}
}

func createTestPods() []*v1.Pod {
	pod1 := &v1.Pod{}
	pod1.Namespace = "other"
	pod1.Status.Phase = v1.PodRunning
	pod2 := &v1.Pod{}
	pod2.Namespace = "testValue"
	pod2.Status.Phase = v1.PodRunning
	pod3 := &v1.Pod{}
	pod3.Namespace = "other"
	pod3.Status.Phase = v1.PodRunning
	return []*v1.Pod{pod1, pod2, pod3}
}
