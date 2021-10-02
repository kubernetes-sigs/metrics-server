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
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/component-base/metrics/testutil"

	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/metrics/pkg/apis/metrics"
)

func TestNodeList(t *testing.T) {
	tcs := []struct {
		name        string
		nodes       []*v1.Node
		listOptions *metainternalversion.ListOptions
		wantNodes   []string
	}{
		{
			name:      "No error",
			nodes:     createTestNodes(),
			wantNodes: []string{"node1", "node2", "node3"},
		},
		{
			name:  "Empty response",
			nodes: nil,
		},
		{
			name:  "With FieldOptions",
			nodes: createTestNodes(),
			listOptions: &metainternalversion.ListOptions{
				FieldSelector: fields.SelectorFromSet(map[string]string{
					"metadata.name": "node2",
				}),
			},
			wantNodes: []string{"node2"},
		},
		{
			name:  "With Label selectors",
			nodes: createTestNodes(),
			listOptions: &metainternalversion.ListOptions{
				LabelSelector: labels.SelectorFromSet(map[string]string{
					"labelKey": "labelValue",
				}),
			},
			wantNodes: []string{"node1"},
		},
		{
			name:  "With both fields and label selectors",
			nodes: createTestNodes(),
			listOptions: &metainternalversion.ListOptions{
				FieldSelector: fields.SelectorFromSet(map[string]string{
					"metadata.name": "node3",
				}),
				LabelSelector: labels.SelectorFromSet(map[string]string{
					"labelKey": "otherValue",
				}),
			},
			wantNodes: []string{"node3"},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// setup
			r := NewTestNodeStorage(tc.nodes, nil)

			// execute
			got, err := r.List(genericapirequest.NewContext(), tc.listOptions)

			// assert
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			res := got.(*metrics.NodeMetricsList)
			if len(res.Items) != len(tc.wantNodes) {
				t.Fatalf("len(res.Items) != %d, got: %d", len(tc.wantNodes), len(res.Items))
			}
			for i := range res.Items {
				testNode(t, res.Items[i], tc.wantNodes[i])
			}
		})
	}
}

func TestNodeList_ConvertToTable(t *testing.T) {
	// setup
	r := NewTestNodeStorage(createTestNodes(), nil)

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
		res.ColumnDefinitions[1].Name != "res1" || res.ColumnDefinitions[2].Name != "Window" ||
		res.Rows[0].Cells[0] != "node1" ||
		res.Rows[0].Cells[1] != "10m" ||
		res.Rows[0].Cells[2] != "1µs" ||
		res.Rows[1].Cells[0] != "node2" ||
		res.Rows[1].Cells[1] != "0" ||
		res.Rows[1].Cells[2] != "2µs" ||
		res.Rows[2].Cells[0] != "node3" ||
		res.Rows[2].Cells[1] != "0" ||
		res.Rows[2].Cells[2] != "3µs" {
		t.Errorf("Got unexpected object: %+v", res)
	}
}

func TestNodeList_Monitoring(t *testing.T) {
	c := &fakeClock{}
	myClock = c

	metricFreshness.Create(nil)
	metricFreshness.Reset()

	r := NewTestNodeStorage(createTestNodes(), nil)
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
type fakeNodeLister struct {
	resp interface{}
	err  error
}

func (pl fakeNodeLister) List(selector labels.Selector) (ret []*v1.Node, err error) {
	data := pl.resp.([]*v1.Node)
	res := []*v1.Node{}
	for _, node := range data {
		if selector.Matches(labels.Set(node.Labels)) {
			res = append(res, node)
		}
	}
	return res, pl.err
}
func (pl fakeNodeLister) Get(name string) (*v1.Node, error) {
	return pl.resp.(*v1.Node), pl.err
}

type fakeNodeMetricsGetter struct {
	time      []TimeInfo
	resources []v1.ResourceList
}

var _ NodeMetricsGetter = (*fakeNodeMetricsGetter)(nil)

func (mp fakeNodeMetricsGetter) GetNodeMetrics(nodes ...string) ([]TimeInfo, []v1.ResourceList, error) {
	return mp.time, mp.resources, nil
}

func NewTestNodeStorage(resp interface{}, err error) *nodeMetrics {
	return &nodeMetrics{
		nodeLister: fakeNodeLister{
			resp: resp,
			err:  err,
		},
		metrics: fakeNodeMetricsGetter{
			time: []TimeInfo{
				{Timestamp: myClock.Now(), Window: 1000},
				{Timestamp: myClock.Now(), Window: 2000},
				{Timestamp: myClock.Now(), Window: 3000},
			},
			resources: []v1.ResourceList{
				{"res1": resource.MustParse("10m")},
				{"res2": resource.MustParse("5Mi")},
				{"res3": resource.MustParse("1")},
			},
		},
	}
}

func testNode(t *testing.T, got metrics.NodeMetrics, wantName string) {
	t.Helper()
	if got.Name != wantName {
		t.Errorf(`Name != "%s", got: %+v`, wantName, got.Name)
	}
	wantLabels := nodeLabels(wantName)
	if diff := cmp.Diff(got.Labels, wantLabels); diff != "" {
		t.Errorf(`Labels != %+v, diff: %s`, wantLabels, diff)
	}
}

func createTestNodes() []*v1.Node {
	node1 := &v1.Node{}
	node1.Name = "node1"
	node1.Labels = nodeLabels(node1.Name)
	node2 := &v1.Node{}
	node2.Name = "node2"
	node2.Labels = nodeLabels(node2.Name)
	node3 := &v1.Node{}
	node3.Name = "node3"
	node3.Labels = nodeLabels(node3.Name)
	return []*v1.Node{node1, node2, node3}
}

func nodeLabels(name string) map[string]string {
	labels := map[string]string{}
	switch name {
	case "node1":
		labels["labelKey"] = "labelValue"
	case "node2":
		labels["otherKey"] = "labelValue"
	case "node3":
		labels["labelKey"] = "otherValue"
	}
	return labels
}
