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

package app

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/glog"

	"github.com/kubernetes-incubator/metrics-server/pkg/provider"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/rest"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/metrics/pkg/apis/metrics"
	_ "k8s.io/metrics/pkg/apis/metrics/install"
)

// kubernetesCadvisorWindow is the max window used by cAdvisor for calculating
// CPU usage rate.  While it can vary, it's no more than this number, but may be
// as low as half this number (when working with no backoff).  It would be really
// nice if the kubelet told us this in the summary API...
var kubernetesCadvisorWindow = 30 * time.Second

type MetricStorage struct {
	groupResource schema.GroupResource
	prov          provider.NodeMetricsProvider
	nodeLister    v1listers.NodeLister
}

var _ rest.KindProvider = &MetricStorage{}
var _ rest.Storage = &MetricStorage{}
var _ rest.Getter = &MetricStorage{}
var _ rest.Lister = &MetricStorage{}
var _ rest.Scoper = &MetricStorage{}

func NewStorage(groupResource schema.GroupResource, prov provider.NodeMetricsProvider, nodeLister v1listers.NodeLister) *MetricStorage {
	return &MetricStorage{
		groupResource: groupResource,
		prov:          prov,
		nodeLister:    nodeLister,
	}
}

// Storage interface
func (m *MetricStorage) New() runtime.Object {
	return &metrics.NodeMetrics{}
}

// KindProvider interface
func (m *MetricStorage) Kind() string {
	return "NodeMetrics"
}

// Lister interface
func (m *MetricStorage) NewList() runtime.Object {
	return &metrics.NodeMetricsList{}
}

// Lister interface
func (m *MetricStorage) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	labelSelector := labels.Everything()
	if options != nil && options.LabelSelector != nil {
		labelSelector = options.LabelSelector
	}
	nodes, err := m.nodeLister.ListWithPredicate(func(node *v1.Node) bool {
		if labelSelector.Empty() {
			return true
		}
		return labelSelector.Matches(labels.Set(node.Labels))
	})
	if err != nil {
		errMsg := fmt.Errorf("Error while listing nodes: %v", err)
		glog.Error(errMsg)
		return &metrics.NodeMetricsList{}, errMsg
	}

	res := metrics.NodeMetricsList{}
	for _, node := range nodes {
		nodeMetrics, err := m.getNodeMetrics(node.Name)
		if err != nil {
			glog.Errorf("unable to fetch node metrics for node %q: %v", node.Name, err)
			continue
		}
		res.Items = append(res.Items, *nodeMetrics)
	}
	return &res, nil
}

func (m *MetricStorage) Get(ctx context.Context, name string, opts *metav1.GetOptions) (runtime.Object, error) {
	nodeMetrics, err := m.getNodeMetrics(name)
	if err != nil {
		glog.Errorf("unable to fetch node metrics for node %q: %v", name, err)
		return nil, errors.NewNotFound(m.groupResource, name)
	}

	return nodeMetrics, nil
}

func (m *MetricStorage) getNodeMetrics(name string) (*metrics.NodeMetrics, error) {
	ts, usage, err := m.prov.GetNodeMetrics(name)
	if err != nil {
		return nil, err
	}

	return &metrics.NodeMetrics{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(time.Now()),
		},
		Timestamp: metav1.NewTime(ts),
		Window:    metav1.Duration{Duration: kubernetesCadvisorWindow},
		Usage:     usage,
	}, nil
}

func (m *MetricStorage) NamespaceScoped() bool {
	return false
}
