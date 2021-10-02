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
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/metrics"
	_ "k8s.io/metrics/pkg/apis/metrics/install"
)

type podMetrics struct {
	groupResource schema.GroupResource
	metrics       PodMetricsGetter
	podLister     v1listers.PodLister
}

var _ rest.KindProvider = &podMetrics{}
var _ rest.Storage = &podMetrics{}
var _ rest.Getter = &podMetrics{}
var _ rest.Lister = &podMetrics{}
var _ rest.TableConvertor = &podMetrics{}
var _ rest.Scoper = &podMetrics{}

func newPodMetrics(groupResource schema.GroupResource, metrics PodMetricsGetter, podLister v1listers.PodLister) *podMetrics {
	return &podMetrics{
		groupResource: groupResource,
		metrics:       metrics,
		podLister:     podLister,
	}
}

// New implements rest.Storage interface
func (m *podMetrics) New() runtime.Object {
	return &metrics.PodMetrics{}
}

// Kind implements rest.KindProvider interface
func (m *podMetrics) Kind() string {
	return "PodMetrics"
}

// NewList implements rest.Lister interface
func (m *podMetrics) NewList() runtime.Object {
	return &metrics.PodMetricsList{}
}

// List implements rest.Lister interface
func (m *podMetrics) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	labelSelector := labels.Everything()
	if options != nil && options.LabelSelector != nil {
		labelSelector = options.LabelSelector
	}

	namespace := genericapirequest.NamespaceValue(ctx)
	pods, err := m.podLister.Pods(namespace).List(labelSelector)
	if err != nil {
		klog.ErrorS(err, "Failed listing pods", "labelSelector", labelSelector, "namespace", klog.KRef("", namespace))
		return &metrics.PodMetricsList{}, fmt.Errorf("failed listing pods: %w", err)
	}

	// currently the PodLister API does not support filtering using FieldSelectors, we have to filter manually
	if options != nil && options.FieldSelector != nil {
		newPods := make([]*corev1.Pod, 0, len(pods))
		fields := make(fields.Set, 2)
		for _, pod := range pods {
			for k := range fields {
				delete(fields, k)
			}
			fieldsSet := generic.AddObjectMetaFieldsSet(fields, &pod.ObjectMeta, true)
			if !options.FieldSelector.Matches(fieldsSet) {
				continue
			}
			newPods = append(newPods, pod)
		}
		pods = newPods
	}

	metricsItems, err := m.getMetrics(pods...)
	if err != nil {
		klog.ErrorS(err, "Failed reading pods metrics", "labelSelector", labelSelector, "namespace", klog.KRef("", namespace))
		return &metrics.PodMetricsList{}, fmt.Errorf("failed reading pods metrics: %w", err)
	}

	if options != nil && options.FieldSelector != nil {
		newMetrics := make([]metrics.PodMetrics, 0, len(metricsItems))
		fields := make(fields.Set, 2)
		for _, metric := range metricsItems {
			for k := range fields {
				delete(fields, k)
			}
			fieldsSet := generic.AddObjectMetaFieldsSet(fields, &metric.ObjectMeta, true)
			if !options.FieldSelector.Matches(fieldsSet) {
				continue
			}
			newMetrics = append(newMetrics, metric)
		}
		metricsItems = newMetrics
	}

	// maintain the same ordering invariant as the Kube API would over pods
	sort.Slice(metricsItems, func(i, j int) bool {
		if metricsItems[i].Namespace != metricsItems[j].Namespace {
			return metricsItems[i].Namespace < metricsItems[j].Namespace
		}
		return metricsItems[i].Name < metricsItems[j].Name
	})

	return &metrics.PodMetricsList{Items: metricsItems}, nil
}

// Get implements rest.Getter interface
func (m *podMetrics) Get(ctx context.Context, name string, opts *metav1.GetOptions) (runtime.Object, error) {
	namespace := genericapirequest.NamespaceValue(ctx)

	pod, err := m.podLister.Pods(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			// return not-found errors directly
			return &metrics.PodMetrics{}, err
		}
		klog.ErrorS(err, "Failed getting pod", "pod", klog.KRef(namespace, name))
		return &metrics.PodMetrics{}, fmt.Errorf("failed getting pod: %w", err)
	}
	if pod == nil {
		return &metrics.PodMetrics{}, errors.NewNotFound(corev1.Resource("pods"), fmt.Sprintf("%s/%s", namespace, name))
	}

	podMetrics, err := m.getMetrics(pod)
	if err != nil {
		klog.ErrorS(err, "Failed reading pod metrics", "pod", klog.KRef(namespace, name))
		return nil, fmt.Errorf("failed pod metrics: %w", err)
	}
	if len(podMetrics) == 0 {
		return nil, errors.NewNotFound(m.groupResource, fmt.Sprintf("%s/%s", namespace, name))
	}
	return &podMetrics[0], nil
}

// ConvertToTable implements rest.TableConvertor interface
func (m *podMetrics) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1beta1.Table, error) {
	var table metav1beta1.Table

	switch t := object.(type) {
	case *metrics.PodMetrics:
		table.ResourceVersion = t.ResourceVersion
		table.SelfLink = t.SelfLink
		addPodMetricsToTable(&table, *t)
	case *metrics.PodMetricsList:
		table.ResourceVersion = t.ResourceVersion
		table.SelfLink = t.SelfLink
		table.Continue = t.Continue
		addPodMetricsToTable(&table, t.Items...)
	default:
	}

	return &table, nil
}

func (m *podMetrics) getMetrics(pods ...*corev1.Pod) ([]metrics.PodMetrics, error) {
	ms, err := m.metrics.GetPodMetrics(pods...)
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		metricFreshness.WithLabelValues().Observe(myClock.Since(m.Timestamp.Time).Seconds())
	}
	return ms, nil
}

// NamespaceScoped implements rest.Scoper interface
func (m *podMetrics) NamespaceScoped() bool {
	return true
}
