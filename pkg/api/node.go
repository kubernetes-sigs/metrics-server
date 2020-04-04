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
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog"
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

func newNodeMetrics(groupResource schema.GroupResource, metrics NodeMetricsGetter, nodeLister v1listers.NodeLister) *nodeMetrics {
	return &nodeMetrics{
		groupResource: groupResource,
		metrics:       metrics,
		nodeLister:    nodeLister,
	}
}

// Storage interface
func (m *nodeMetrics) New() runtime.Object {
	return &metrics.NodeMetrics{}
}

// KindProvider interface
func (m *nodeMetrics) Kind() string {
	return "NodeMetrics"
}

// Lister interface
func (m *nodeMetrics) NewList() runtime.Object {
	return &metrics.NodeMetricsList{}
}

// Lister interface
func (m *nodeMetrics) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	labelSelector := labels.Everything()
	fieldSelector := fields.Everything()
	if options != nil && options.LabelSelector != nil {
		labelSelector = options.LabelSelector
	}
	if options != nil && options.FieldSelector != nil {
		fieldSelector = options.FieldSelector
	}
	nodes, err := m.nodeLister.ListWithPredicate(func(node *v1.Node) bool {
		if labelSelector.Empty() && fieldSelector.Empty() {
			return true
		}
		fieldsSet := generic.AddObjectMetaFieldsSet(make(fields.Set, 2), &node.ObjectMeta, true)
		return labelSelector.Matches(labels.Set(node.Labels)) && fieldSelector.Matches(fieldsSet)
	})
	if err != nil {
		errMsg := fmt.Errorf("Error while listing nodes for selector %v: %v", labelSelector, err)
		klog.Error(errMsg)
		return &metrics.NodeMetricsList{}, errMsg
	}

	names := make([]string, len(nodes))
	for i, node := range nodes {
		names[i] = node.Name
	}
	// maintain the same ordering invariant as the Kube API would over nodes
	sort.Strings(names)

	metricsItems, err := m.getNodeMetrics(names...)
	if err != nil {
		errMsg := fmt.Errorf("Error while fetching node metrics for selector %v: %v", labelSelector, err)
		klog.Error(errMsg)
		return &metrics.NodeMetricsList{}, errMsg
	}

	return &metrics.NodeMetricsList{Items: metricsItems}, nil
}

func (m *nodeMetrics) Get(ctx context.Context, name string, opts *metav1.GetOptions) (runtime.Object, error) {
	nodeMetrics, err := m.getNodeMetrics(name)
	if err == nil && len(nodeMetrics) == 0 {
		err = fmt.Errorf("no metrics known for node %q", name)
	}
	if err != nil {
		klog.Errorf("unable to fetch node metrics for node %q: %v", name, err)
		return nil, errors.NewNotFound(m.groupResource, name)
	}

	return &nodeMetrics[0], nil
}

func (m *nodeMetrics) getNodeMetrics(names ...string) ([]metrics.NodeMetrics, error) {
	timestamps, usages, err := m.metrics.GetNodeMetrics(names...)
	if err != nil {
		return nil, err
	}

	res := make([]metrics.NodeMetrics, 0, len(names))

	for i, name := range names {
		if usages[i] == nil {
			klog.Errorf("unable to fetch node metrics for node %q: no metrics known for node", name)

			continue
		}
		res = append(res, metrics.NodeMetrics{
			ObjectMeta: metav1.ObjectMeta{
				Name:              name,
				CreationTimestamp: metav1.NewTime(time.Now()),
			},
			Timestamp: metav1.NewTime(timestamps[i].Timestamp),
			Window:    metav1.Duration{Duration: timestamps[i].Window},
			Usage:     usages[i],
		})
	}

	return res, nil
}

func (m *nodeMetrics) NamespaceScoped() bool {
	return false
}
