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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	nodeSelector  []labels.Requirement
}

var _ rest.KindProvider = &nodeMetrics{}
var _ rest.Storage = &nodeMetrics{}
var _ rest.Getter = &nodeMetrics{}
var _ rest.Lister = &nodeMetrics{}
var _ rest.Scoper = &nodeMetrics{}
var _ rest.TableConvertor = &nodeMetrics{}
var _ rest.SingularNameProvider = &nodeMetrics{}

func newNodeMetrics(groupResource schema.GroupResource, metrics NodeMetricsGetter, nodeLister v1listers.NodeLister, nodeSelector []labels.Requirement) *nodeMetrics {
	return &nodeMetrics{
		groupResource: groupResource,
		metrics:       metrics,
		nodeLister:    nodeLister,
		nodeSelector:  nodeSelector,
	}
}

// New implements rest.Storage interface
func (m *nodeMetrics) New() runtime.Object {
	return &metrics.NodeMetrics{}
}

// Destroy implements rest.Storage interface
func (m *nodeMetrics) Destroy() {
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
	nodes, err := m.nodes(ctx, options)
	if err != nil {
		return &metrics.NodeMetricsList{}, err
	}

	ms, err := m.getMetrics(nodes...)
	if err != nil {
		klog.ErrorS(err, "Failed reading nodes metrics")
		return &metrics.NodeMetricsList{}, fmt.Errorf("failed reading nodes metrics: %w", err)
	}
	return &metrics.NodeMetricsList{Items: ms}, nil
}

func (m *nodeMetrics) nodes(ctx context.Context, options *metainternalversion.ListOptions) ([]*corev1.Node, error) {
	labelSelector := labels.Everything()
	if options != nil && options.LabelSelector != nil {
		labelSelector = options.LabelSelector
	}
	if m.nodeSelector != nil {
		labelSelector = labelSelector.Add(m.nodeSelector...)
	}
	nodes, err := m.nodeLister.List(labelSelector)
	if err != nil {
		klog.ErrorS(err, "Failed listing nodes", "labelSelector", labelSelector)
		return nil, fmt.Errorf("failed listing nodes: %w", err)
	}
	if options != nil && options.FieldSelector != nil {
		nodes = filterNodes(nodes, options.FieldSelector)
	}
	return nodes, nil
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
	ms, err := m.getMetrics(node)
	if err != nil {
		klog.ErrorS(err, "Failed reading node metrics", "node", klog.KRef("", name))
		return nil, fmt.Errorf("failed reading node metrics: %w", err)
	}
	if len(ms) == 0 {
		return nil, errors.NewNotFound(m.groupResource, name)
	}
	return &ms[0], nil
}

// ConvertToTable implements rest.TableConvertor interface
func (m *nodeMetrics) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1beta1.Table, error) {
	var table metav1beta1.Table

	switch t := object.(type) {
	case *metrics.NodeMetrics:
		table.ResourceVersion = t.ResourceVersion
		table.SelfLink = t.SelfLink //nolint:staticcheck // keep deprecated field to be backward compatible
		addNodeMetricsToTable(&table, *t)
	case *metrics.NodeMetricsList:
		table.ResourceVersion = t.ResourceVersion
		table.SelfLink = t.SelfLink //nolint:staticcheck // keep deprecated field to be backward compatible
		table.Continue = t.Continue
		addNodeMetricsToTable(&table, t.Items...)
	default:
	}

	return &table, nil
}

func (m *nodeMetrics) getMetrics(nodes ...*corev1.Node) ([]metrics.NodeMetrics, error) {
	ms, err := m.metrics.GetNodeMetrics(nodes...)
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		metricFreshness.WithLabelValues().Observe(myClock.Since(m.Timestamp.Time).Seconds())
	}
	// maintain the same ordering invariant as the Kube API would over nodes
	sort.Slice(ms, func(i, j int) bool {
		return ms[i].Name < ms[j].Name
	})
	return ms, nil
}

// NamespaceScoped implements rest.Scoper interface
func (m *nodeMetrics) NamespaceScoped() bool {
	return false
}

// GetSingularName implements rest.SingularNameProvider interface
func (m *nodeMetrics) GetSingularName() string {
	return "node"
}
