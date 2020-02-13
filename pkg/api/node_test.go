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

package nodemetrics

import (
	"reflect"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	fields "k8s.io/apimachinery/pkg/fields"
	labels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/diff"

	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	clientv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/metrics/pkg/apis/metrics"
	"sigs.k8s.io/metrics-server/pkg/provider"
)

// fakes both PodLister and PodNamespaceLister at once
type fakeNodeLister struct {
	resp interface{}
	err  error
}

func (pl fakeNodeLister) List(selector labels.Selector) (ret []*v1.Node, err error) {
	return pl.resp.([]*v1.Node), pl.err
}
func (pl fakeNodeLister) Get(name string) (*v1.Node, error) {
	return pl.resp.(*v1.Node), pl.err
}
func (pl fakeNodeLister) ListWithPredicate(predicate clientv1.NodeConditionPredicate) ([]*v1.Node, error) {
	data := pl.resp.([]*v1.Node)
	res := []*v1.Node{}
	for _, node := range data {
		if predicate(node) {
			res = append(res, node)
		}
	}
	return res, pl.err
}

type fakeMetricsProvider struct{}

func (mp fakeMetricsProvider) GetNodeMetrics(nodes ...string) ([]provider.TimeInfo, []v1.ResourceList, error) {
	return []provider.TimeInfo{
			{Timestamp: time.Now(), Window: 1000}, {Timestamp: time.Now(), Window: 2000}, {Timestamp: time.Now(), Window: 3000},
		}, []v1.ResourceList{
			{"res1": resource.Quantity{}},
			{"res2": resource.Quantity{}},
			{"res3": resource.Quantity{}},
		}, nil
}

func NewTestStorage(resp interface{}, err error) *MetricStorage {
	return &MetricStorage{
		nodeLister: fakeNodeLister{
			resp: resp,
			err:  err,
		},
		prov: fakeMetricsProvider{},
	}
}

func TestList_NoError(t *testing.T) {
	// setup
	r := NewTestStorage(createTestNodes(), nil)

	// execute
	got, err := r.List(genericapirequest.NewContext(), nil)

	// assert
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	res := got.(*metrics.NodeMetricsList)

	if len(res.Items) != 3 ||
		res.Items[0].Name != "node1" ||
		res.Items[1].Name != "node2" ||
		res.Items[2].Name != "node3" {
		t.Errorf("Got unexpected object: %+v", got)
	}
}

func TestList_EmptyResponse(t *testing.T) {
	// setup
	r := NewTestStorage([]*v1.Node{}, nil)

	// execute
	got, err := r.List(genericapirequest.NewContext(), nil)

	// assert
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expect := &metrics.NodeMetricsList{Items: []metrics.NodeMetrics{}}
	if e, a := expect, got; !reflect.DeepEqual(e, a) {
		t.Errorf("Got unexpected object: %+v", diff.ObjectDiff(e, a))
	}
}

func TestList_WithFieldSelectors(t *testing.T) {
	// setup
	r := NewTestStorage(createTestNodes(), nil)

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
	res := got.(*metrics.NodeMetricsList)

	if len(res.Items) != 1 ||
		res.Items[0].Name != "node2" {
		t.Errorf("Got unexpected object: %+v", got)
	}
}

func TestList_WithLabelSelectors(t *testing.T) {
	// setup
	r := NewTestStorage(createTestNodes(), nil)

	opts := &metainternalversion.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"labelKey": "labelValue",
		}),
	}

	// execute
	got, err := r.List(genericapirequest.NewContext(), opts)

	// assert
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	res := got.(*metrics.NodeMetricsList)

	if len(res.Items) != 1 ||
		res.Items[0].Name != "node1" {
		t.Errorf("Got unexpected object: %+v", got)
	}
}

func TestList_WithLabelAndFieldSelectors(t *testing.T) {
	// setup
	r := NewTestStorage(createTestNodes(), nil)

	opts := &metainternalversion.ListOptions{
		FieldSelector: fields.SelectorFromSet(map[string]string{
			"metadata.namespace": "other",
		}),
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"labelKey": "otherValue",
		}),
	}

	// execute
	got, err := r.List(genericapirequest.NewContext(), opts)

	// assert
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	res := got.(*metrics.NodeMetricsList)

	if len(res.Items) != 1 ||
		res.Items[0].Name != "node3" {
		t.Errorf("Got unexpected object: %+v", got)
	}
}

func createTestNodes() []*v1.Node {
	node1 := &v1.Node{}
	node1.Name = "node1"
	node1.Namespace = "other"
	node1.Labels = map[string]string{
		"labelKey": "labelValue",
	}
	node2 := &v1.Node{}
	node2.Name = "node2"
	node2.Namespace = "testValue"
	node2.Labels = map[string]string{
		"otherKey": "labelValue",
	}
	node3 := &v1.Node{}
	node3.Name = "node3"
	node3.Namespace = "other"
	node3.Labels = map[string]string{
		"labelKey": "otherValue",
	}
	return []*v1.Node{node1, node2, node3}
}
