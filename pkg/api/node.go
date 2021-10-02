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

package api

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/metrics"
	_ "k8s.io/metrics/pkg/apis/metrics/install"
)

type nodeMetrics struct {
	groupResource schema.GroupResource
	metrics       NodeMetricsGetter
	nodeLister    v1listers.NodeLister
}

var _ rest.KindProvider = &nodeMetrics{}
var _ rest.Storage = &nodeMetrics{}
var _ rest.Getter = &nodeMetrics{}
var _ rest.Lister = &nodeMetrics{}
var _ rest.Scoper = &nodeMetrics{}
var _ rest.TableConvertor = &nodeMetrics{}

func newNodeMetrics(groupResource schema.GroupResource, metrics NodeMetricsGetter, nodeLister v1listers.NodeLister) *nodeMetrics {
	return &nodeMetrics{
		groupResource: groupResource,
		metrics:       metrics,
		nodeLister:    nodeLister,
	}
}

// New implements rest.Storage interface
func (m *nodeMetrics) New() runtime.Object {
	return &metrics.NodeMetrics{}
}

// Kind implements rest.KindProvider interface
func (m *nodeMetrics) Kind() string {
	return "NodeMetrics"
}

// NewList implements rest.Lister interface
func (m *nodeMetrics) NewList() runtime.Object {
	return &metrics.NodeMetricsList{}
}

// List implements rest.Lister interface
func (m *nodeMetrics) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	labelSelector := labels.Everything()
	if options != nil && options.LabelSelector != nil {
		labelSelector = options.LabelSelector
	}
	nodes, err := m.nodeLister.List(labelSelector)
	if err != nil {
		klog.ErrorS(err, "Failed listing nodes", "labelSelector", labelSelector)
		return &metrics.NodeMetricsList{}, fmt.Errorf("failed listing nodes: %w", err)
	}

	// maintain the same ordering invariant as the Kube API would over nodes
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})

	ms, err := m.getMetrics(nodes...)
	if err != nil {
		klog.ErrorS(err, "Failed reading nodes metrics", "labelSelector", labelSelector)
		return &metrics.NodeMetricsList{}, fmt.Errorf("failed reading nodes metrics: %w", err)
	}

	if options != nil && options.FieldSelector != nil {
		newMetrics := make([]metrics.NodeMetrics, 0, len(ms))
		fields := make(fields.Set, 2)
		for _, metric := range ms {
			for k := range fields {
				delete(fields, k)
			}
			fieldsSet := generic.AddObjectMetaFieldsSet(fields, &metric.ObjectMeta, false)
			if !options.FieldSelector.Matches(fieldsSet) {
				continue
			}
			newMetrics = append(newMetrics, metric)
		}
		ms = newMetrics
	}

	return &metrics.NodeMetricsList{Items: ms}, nil
}

// Get implements rest.Getter interface
func (m *nodeMetrics) Get(ctx context.Context, name string, opts *metav1.GetOptions) (runtime.Object, error) {
	node, err := m.nodeLister.Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			// return not-found errors directly
			return nil, err
		}
		klog.ErrorS(err, "Failed getting node", "node", klog.KRef("", name))
		return nil, fmt.Errorf("failed getting node: %w", err)
	}
	if node == nil {
		return nil, errors.NewNotFound(m.groupResource, name)
	}
	metrics, err := m.getMetrics(node)
	if err != nil {
		klog.ErrorS(err, "Failed reading node metrics", "node", klog.KRef("", name))
		return nil, fmt.Errorf("failed reading node metrics: %w", err)
	}
	if len(metrics) == 0 {
		return nil, errors.NewNotFound(m.groupResource, name)
	}
	return &metrics[0], nil
}

// ConvertToTable implements rest.TableConvertor interface
func (m *nodeMetrics) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1beta1.Table, error) {
	var table metav1beta1.Table

	switch t := object.(type) {
	case *metrics.NodeMetrics:
		table.ResourceVersion = t.ResourceVersion
		table.SelfLink = t.SelfLink
		addNodeMetricsToTable(&table, *t)
	case *metrics.NodeMetricsList:
		table.ResourceVersion = t.ResourceVersion
		table.SelfLink = t.SelfLink
		table.Continue = t.Continue
		addNodeMetricsToTable(&table, t.Items...)
	default:
	}

	return &table, nil
}

func addNodeMetricsToTable(table *metav1beta1.Table, nodes ...metrics.NodeMetrics) {
	var names []string
	for i, node := range nodes {
		if names == nil {
			for k := range node.Usage {
				names = append(names, string(k))
			}
			sort.Strings(names)

			table.ColumnDefinitions = []metav1beta1.TableColumnDefinition{
				{Name: "Name", Type: "string", Format: "name", Description: "Name of the resource"},
			}
			for _, name := range names {
				table.ColumnDefinitions = append(table.ColumnDefinitions, metav1beta1.TableColumnDefinition{
					Name:   name,
					Type:   "string",
					Format: "quantity",
				})
			}
			table.ColumnDefinitions = append(table.ColumnDefinitions, metav1beta1.TableColumnDefinition{
				Name:   "Window",
				Type:   "string",
				Format: "duration",
			})
		}
		row := make([]interface{}, 0, len(names)+1)
		row = append(row, node.Name)
		for _, name := range names {
			v := node.Usage[corev1.ResourceName(name)]
			row = append(row, v.String())
		}
		row = append(row, node.Window.Duration.String())
		table.Rows = append(table.Rows, metav1beta1.TableRow{
			Cells:  row,
			Object: runtime.RawExtension{Object: &nodes[i]},
		})
	}
}

func (m *nodeMetrics) getMetrics(nodes ...*corev1.Node) ([]metrics.NodeMetrics, error) {
	ms, err := m.metrics.GetNodeMetrics(nodes...)
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		metricFreshness.WithLabelValues().Observe(myClock.Since(m.Timestamp.Time).Seconds())
	}
	return ms, nil
}

// NamespaceScoped implements rest.Scoper interface
func (m *nodeMetrics) NamespaceScoped() bool {
	return false
}
