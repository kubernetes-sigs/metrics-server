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

package podmetrics

import (
	"context"
	"fmt"
	"time"

	"github.com/kubernetes-incubator/metrics-server/pkg/provider"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apitypes "k8s.io/apimachinery/pkg/types"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog"
	"k8s.io/metrics/pkg/apis/metrics"
	_ "k8s.io/metrics/pkg/apis/metrics/install"
)

type MetricStorage struct {
	groupResource schema.GroupResource
	prov          provider.PodMetricsProvider
	podLister     v1listers.PodLister
}

var _ rest.KindProvider = &MetricStorage{}
var _ rest.Storage = &MetricStorage{}
var _ rest.Getter = &MetricStorage{}
var _ rest.Lister = &MetricStorage{}

func NewStorage(groupResource schema.GroupResource, prov provider.PodMetricsProvider, podLister v1listers.PodLister) *MetricStorage {
	return &MetricStorage{
		groupResource: groupResource,
		prov:          prov,
		podLister:     podLister,
	}
}

// Storage interface
func (m *MetricStorage) New() runtime.Object {
	return &metrics.PodMetrics{}
}

// KindProvider interface
func (m *MetricStorage) Kind() string {
	return "PodMetrics"
}

// Lister interface
func (m *MetricStorage) NewList() runtime.Object {
	return &metrics.PodMetricsList{}
}

// Lister interface
func (m *MetricStorage) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	labelSelector := labels.Everything()
	if options != nil && options.LabelSelector != nil {
		labelSelector = options.LabelSelector
	}

	namespace := genericapirequest.NamespaceValue(ctx)
	pods, err := m.podLister.Pods(namespace).List(labelSelector)
	if err != nil {
		errMsg := fmt.Errorf("Error while listing pods for selector %v in namespace %q: %v", labelSelector, namespace, err)
		klog.Error(errMsg)
		return &metrics.PodMetricsList{}, errMsg
	}

	// currently the PodLister API does not support filtering using FieldSelectors, we have to filter manually
	if options != nil && options.FieldSelector != nil {
		for i := len(pods) - 1; i >= 0; i-- {
			fieldsSet := generic.AddObjectMetaFieldsSet(make(fields.Set, 2), &pods[i].ObjectMeta, true)
			if !options.FieldSelector.Matches(fieldsSet) {
				pods = append(pods[:i], pods[i+1:]...)
			}
		}
	}

	metricsItems, err := m.getPodMetrics(pods...)
	if err != nil {
		errMsg := fmt.Errorf("Error while fetching pod metrics for selector %v in namespace %q: %v", labelSelector, namespace, err)
		klog.Error(errMsg)
		return &metrics.PodMetricsList{}, errMsg
	}

	return &metrics.PodMetricsList{Items: metricsItems}, nil
}

// Getter interface
func (m *MetricStorage) Get(ctx context.Context, name string, opts *metav1.GetOptions) (runtime.Object, error) {
	namespace := genericapirequest.NamespaceValue(ctx)

	pod, err := m.podLister.Pods(namespace).Get(name)
	if err != nil {
		errMsg := fmt.Errorf("Error while getting pod %v: %v", name, err)
		klog.Error(errMsg)
		if errors.IsNotFound(err) {
			// return not-found errors directly
			return &metrics.PodMetrics{}, err
		}
		return &metrics.PodMetrics{}, errMsg
	}
	if pod == nil {
		return &metrics.PodMetrics{}, errors.NewNotFound(v1.Resource("pods"), fmt.Sprintf("%v/%v", namespace, name))
	}

	podMetrics, err := m.getPodMetrics(pod)
	if err == nil && len(podMetrics) == 0 {
		err = fmt.Errorf("no metrics known for pod \"%s/%s\"", pod.Namespace, pod.Name)
	}
	if err != nil {
		klog.Errorf("unable to fetch pod metrics for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		return nil, errors.NewNotFound(m.groupResource, fmt.Sprintf("%v/%v", namespace, name))
	}
	return &podMetrics[0], nil
}

func (m *MetricStorage) getPodMetrics(pods ...*v1.Pod) ([]metrics.PodMetrics, error) {
	namespacedNames := make([]apitypes.NamespacedName, len(pods))
	for i, pod := range pods {
		namespacedNames[i] = apitypes.NamespacedName{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		}
	}
	timestamps, containerMetrics, err := m.prov.GetContainerMetrics(namespacedNames...)
	if err != nil {
		return nil, err
	}

	res := make([]metrics.PodMetrics, 0, len(pods))

	for i, pod := range pods {
		if pod.Status.Phase != v1.PodRunning {
			// ignore pod not in Running phase
			continue
		}
		if containerMetrics[i] == nil {
			klog.Errorf("unable to fetch pod metrics for pod %s/%s: no metrics known for pod", pod.Namespace, pod.Name)
			continue
		}

		res = append(res, metrics.PodMetrics{
			ObjectMeta: metav1.ObjectMeta{
				Name:              pod.Name,
				Namespace:         pod.Namespace,
				CreationTimestamp: metav1.NewTime(time.Now()),
			},
			Timestamp:  metav1.NewTime(timestamps[i].Timestamp),
			Window:     metav1.Duration{Duration: timestamps[i].Window},
			Containers: containerMetrics[i],
		})
	}
	return res, nil
}

func (m *MetricStorage) NamespaceScoped() bool {
	return true
}
