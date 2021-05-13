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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apitypes "k8s.io/apimachinery/pkg/types"
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

func newPodMetrics(groupResource schema.GroupResource, metrics PodMetricsGetter, podLister v1listers.PodLister) *podMetrics {
	return &podMetrics{
		groupResource: groupResource,
		metrics:       metrics,
		podLister:     podLister,
	}
}

// Storage interface
func (m *podMetrics) New() runtime.Object {
	return &metrics.PodMetrics{}
}

// KindProvider interface
func (m *podMetrics) Kind() string {
	return "PodMetrics"
}

// Lister interface
func (m *podMetrics) NewList() runtime.Object {
	return &metrics.PodMetricsList{}
}

// Lister interface
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
		newPods := make([]*v1.Pod, 0, len(pods))
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

	// maintain the same ordering invariant as the Kube API would over pods
	sort.Slice(pods, func(i, j int) bool {
		if pods[i].Namespace != pods[j].Namespace {
			return pods[i].Namespace < pods[j].Namespace
		}
		return pods[i].Name < pods[j].Name
	})

	metricsItems, err := m.getPodMetrics(pods...)
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

	return &metrics.PodMetricsList{Items: metricsItems}, nil
}

// Getter interface
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
		return &metrics.PodMetrics{}, errors.NewNotFound(v1.Resource("pods"), fmt.Sprintf("%s/%s", namespace, name))
	}

	podMetrics, err := m.getPodMetrics(pod)
	if err != nil {
		klog.ErrorS(err, "Failed reading pod metrics", "pod", klog.KRef(namespace, name))
		return nil, fmt.Errorf("failed pod metrics: %w", err)
	}
	if len(podMetrics) == 0 {
		return nil, errors.NewNotFound(m.groupResource, fmt.Sprintf("%s/%s", namespace, name))
	}
	return &podMetrics[0], nil
}

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

func addPodMetricsToTable(table *metav1beta1.Table, pods ...metrics.PodMetrics) {
	usage := make(v1.ResourceList, 3)
	var names []string
	for i, pod := range pods {
		for k := range usage {
			delete(usage, k)
		}
		for _, container := range pod.Containers {
			for k, v := range container.Usage {
				u := usage[k]
				u.Add(v)
				usage[k] = u
			}
		}
		if names == nil {
			for k := range usage {
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
		row = append(row, pod.Name)
		for _, name := range names {
			v := usage[v1.ResourceName(name)]
			row = append(row, v.String())
		}
		row = append(row, pod.Window.Duration.String())
		table.Rows = append(table.Rows, metav1beta1.TableRow{
			Cells:  row,
			Object: runtime.RawExtension{Object: &pods[i]},
		})
	}
}

func (m *podMetrics) getPodMetrics(pods ...*v1.Pod) ([]metrics.PodMetrics, error) {
	namespacedNames := make([]apitypes.NamespacedName, len(pods))
	for i, pod := range pods {
		namespacedNames[i] = apitypes.NamespacedName{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		}
	}
	timestamps, containerMetrics, err := m.metrics.GetPodMetrics(namespacedNames...)
	if err != nil {
		return nil, err
	}

	res := make([]metrics.PodMetrics, 0, len(pods))

	for i, pod := range pods {
		if containerMetrics[i] == nil {
			continue
		}

		res = append(res, metrics.PodMetrics{
			ObjectMeta: metav1.ObjectMeta{
				Name:              pod.Name,
				Namespace:         pod.Namespace,
				CreationTimestamp: metav1.NewTime(myClock.Now()),
				Labels:            pod.Labels,
			},
			Timestamp:  metav1.NewTime(timestamps[i].Timestamp),
			Window:     metav1.Duration{Duration: timestamps[i].Window},
			Containers: containerMetrics[i],
		})
		metricFreshness.WithLabelValues().Observe(myClock.Since(timestamps[i].Timestamp).Seconds())
	}
	return res, nil
}

func (m *podMetrics) NamespaceScoped() bool {
	return true
}
