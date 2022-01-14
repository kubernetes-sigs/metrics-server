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
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/component-base/metrics/testutil"
	"k8s.io/metrics/pkg/apis/metrics"
)

func TestNodeList(t *testing.T) {
	tcs := []struct {
		name        string
		listOptions *metainternalversion.ListOptions
		listerError error
		wantNodes   []string
		wantError   bool
	}{
		{
			name:      "No error",
			wantNodes: []string{"node1", "node2", "node3"},
		},
		{
			name: "Empty response",
			listOptions: &metainternalversion.ListOptions{
				FieldSelector: fields.SelectorFromSet(map[string]string{
					"metadata.name": "node4",
				}),
			},
		},
		{
			name: "With FieldOptions",
			listOptions: &metainternalversion.ListOptions{
				FieldSelector: fields.SelectorFromSet(map[string]string{
					"metadata.name": "node2",
				}),
			},
			wantNodes: []string{"node2"},
		},
		{
			name: "With Label selectors",
			listOptions: &metainternalversion.ListOptions{
				LabelSelector: labels.SelectorFromSet(map[string]string{
					"labelKey": "labelValue",
				}),
			},
			wantNodes: []string{"node1"},
		},
		{
			name: "With both fields and label selectors",
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
		{
			name:        "Lister error",
			listerError: fmt.Errorf("lister error"),
			wantNodes:   nil,
			wantError:   true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// setup
			r := NewTestNodeStorage(tc.listerError)

			// execute
			got, err := r.List(genericapirequest.NewContext(), tc.listOptions)

			// assert
			if (err != nil) != tc.wantError {
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

func TestNodeGet(t *testing.T) {
	tcs := []struct {
		name        string
		get         string
		listerError error
		wantNode    string
		wantError   bool
	}{
		{
			name:     "Normal",
			get:      "node1",
			wantNode: "node1",
		},
		{
			name:      "Empty response",
			get:       "node4",
			wantError: true,
		},
		{
			name:        "Lister error",
			get:         "node1",
			listerError: fmt.Errorf("lister error"),
			wantError:   true,
		},
		{
			name:      "Node without metrics",
			get:       "node4",
			wantError: true,
		},
		{
			name:      "Node doesn't exist",
			get:       "node5",
			wantError: true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// setup
			r := NewTestNodeStorage(tc.listerError)

			// execute
			got, err := r.Get(genericapirequest.NewContext(), tc.get, nil)

			// assert
			if (err != nil) != tc.wantError {
				t.Fatalf("Unexpected error: %v", err)
			}
			if tc.wantError {
				return
			}
			res := got.(*metrics.NodeMetrics)
			testNode(t, *res, tc.wantNode)
		})
	}
}

func TestNodeList_Monitoring(t *testing.T) {
	c := &fakeClock{}
	myClock = c

	metricFreshness.Create(nil)
	metricFreshness.Reset()

	r := NewTestNodeStorage(nil)
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
	data []*corev1.Node
	err  error
}

func (pl fakeNodeLister) List(selector labels.Selector) ([]*corev1.Node, error) {
	if pl.err != nil {
		return nil, pl.err
	}
	res := []*corev1.Node{}
	for _, node := range pl.data {
		if selector.Matches(labels.Set(node.Labels)) {
			res = append(res, node)
		}
	}
	return res, nil
}
func (pl fakeNodeLister) Get(name string) (*corev1.Node, error) {
	if pl.err != nil {
		return nil, pl.err
	}
	for _, node := range pl.data {
		if node.Name == name {
			return node, nil
		}
	}
	return nil, nil
}

type fakeNodeMetricsGetter struct {
	now time.Time
}

var _ NodeMetricsGetter = (*fakeNodeMetricsGetter)(nil)

func (mp fakeNodeMetricsGetter) GetNodeMetrics(nodes ...*corev1.Node) ([]metrics.NodeMetrics, error) {
	ms := make([]metrics.NodeMetrics, 0, len(nodes))
	for _, node := range nodes {
		switch node.Name {
		case "node1":
			ms = append(ms, metrics.NodeMetrics{
				ObjectMeta: metav1.ObjectMeta{Name: node.Name, Labels: node.Labels},
				Timestamp:  metav1.Time{Time: mp.now},
				Window:     metav1.Duration{Duration: 1000},
				Usage:      corev1.ResourceList{"res1": resource.MustParse("10m")},
			})
		case "node2":
			ms = append(ms, metrics.NodeMetrics{
				ObjectMeta: metav1.ObjectMeta{Name: node.Name, Labels: node.Labels},
				Timestamp:  metav1.Time{Time: mp.now},
				Window:     metav1.Duration{Duration: 2000},
				Usage:      corev1.ResourceList{"res1": resource.MustParse("5Mi")},
			})
		case "node3":
			ms = append(ms, metrics.NodeMetrics{
				ObjectMeta: metav1.ObjectMeta{Name: node.Name, Labels: node.Labels},
				Timestamp:  metav1.Time{Time: mp.now},
				Window:     metav1.Duration{Duration: 3000},
				Usage:      corev1.ResourceList{"res1": resource.MustParse("1")},
			})
		}
	}
	return ms, nil
}

func NewTestNodeStorage(listerError error) *nodeMetrics {
	return &nodeMetrics{
		nodeLister: fakeNodeLister{
			data: createTestNodes(),
			err:  listerError,
		},
		metrics: fakeNodeMetricsGetter{now: myClock.Now()},
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

func createTestNodes() []*corev1.Node {
	node1 := &corev1.Node{}
	node1.Name = "node1"
	node1.Labels = nodeLabels(node1.Name)
	node2 := &corev1.Node{}
	node2.Name = "node2"
	node2.Labels = nodeLabels(node2.Name)
	node3 := &corev1.Node{}
	node3.Name = "node3"
	node3.Labels = nodeLabels(node3.Name)
	node4 := &corev1.Node{}
	node4.Name = "node4"
	node4.Labels = nodeLabels(node4.Name)
	return []*corev1.Node{node1, node2, node3, node4}
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
	case "node4":
		labels["otherKey"] = "otherValue"
	}
	return labels
}
