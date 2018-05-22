// Copyright 2016 Google Inc. All Rights Reserved.
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
	"fmt"
	"time"

	"github.com/golang/glog"

	api "github.com/kubernetes-incubator/metrics-server/metrics/api/v1alpha1"
	"github.com/kubernetes-incubator/metrics-server/metrics/core"
	metricsink "github.com/kubernetes-incubator/metrics-server/metrics/sinks/metric"
	"github.com/kubernetes-incubator/metrics-server/metrics/storage/util"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/metrics/pkg/apis/metrics"
	_ "k8s.io/metrics/pkg/apis/metrics/install"
)

type MetricStorage struct {
	groupResource schema.GroupResource
	metricSink    *metricsink.MetricSink
	podLister     v1listers.PodLister
}

var _ rest.KindProvider = &MetricStorage{}
var _ rest.Storage = &MetricStorage{}
var _ rest.GetterWithOptions = &MetricStorage{}
var _ rest.Lister = &MetricStorage{}

func NewStorage(groupResource schema.GroupResource, metricSink *metricsink.MetricSink, podLister v1listers.PodLister) *MetricStorage {
	return &MetricStorage{
		groupResource: groupResource,
		metricSink:    metricSink,
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
func (m *MetricStorage) List(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	labelSelector := labels.Everything()
	if options != nil && options.LabelSelector != nil {
		labelSelector = options.LabelSelector
	}
	namespace := genericapirequest.NamespaceValue(ctx)
	pods, err := m.podLister.Pods(namespace).List(labelSelector)
	if err != nil {
		errMsg := fmt.Errorf("Error while listing pods for selector %v: %v", labelSelector, err)
		glog.Error(errMsg)
		return &metrics.PodMetricsList{}, errMsg
	}

	res := metrics.PodMetricsList{}
	for _, pod := range pods {
		if podMetrics := m.getPodMetrics(pod); podMetrics != nil {
			res.Items = append(res.Items, *podMetrics)
		} else {
			glog.Infof("No metrics for pod %s/%s", pod.Namespace, pod.Name)
		}
	}
	return &res, nil
}

// GetterWithOptions interface
func (m *MetricStorage) Get(ctx genericapirequest.Context, name string, options runtime.Object) (runtime.Object, error) {
	opts, ok := options.(*api.MetricsOptions)
	if !ok {
		return nil, fmt.Errorf("invalid options object: %#v", options)
	}

	namespace := genericapirequest.NamespaceValue(ctx)

	pod, err := m.podLister.Pods(namespace).Get(name)
	if err != nil {
		errMsg := fmt.Errorf("Error while getting pod %v: %v", name, err)
		glog.Error(errMsg)
		return &metrics.PodMetrics{}, errMsg
	}
	if pod == nil {
		return &metrics.PodMetrics{}, errors.NewNotFound(v1.Resource("pods"), fmt.Sprintf("%v/%v", namespace, name))
	}

	if opts.SinceSeconds != nil {
		podMetricsList := m.getPodHistoricalMetrics(pod, opts.SinceSeconds)
		if podMetricsList == nil {
			return &metrics.PodMetricsList{}, errors.NewNotFound(m.groupResource, fmt.Sprintf("%v/%v", namespace, name))
		}
		return podMetricsList, nil
	} else {
		podMetrics := m.getPodMetrics(pod)
		if podMetrics == nil {
			return &metrics.PodMetrics{}, errors.NewNotFound(m.groupResource, fmt.Sprintf("%v/%v", namespace, name))
		}
		return podMetrics, nil
	}
}

func (m *MetricStorage) NewGetOptions() (runtime.Object, bool, string) {
	return &api.MetricsOptions{}, true, "path"
}

func (m *MetricStorage) getPodHistoricalMetrics(pod *v1.Pod, SinceSeconds *int64) *metrics.PodMetricsList {
	keys := make([]string, 0)
	for _, container := range pod.Spec.Containers {
		keys = append(keys, core.PodContainerKey(pod.Namespace, pod.Name, container.Name))
	}

	metricsList := m.metricSink.GetHistoricalMetrics([]string{core.MetricCpuUsageRate.MetricDescriptor.Name, core.MetricMemoryWorkingSet.MetricDescriptor.Name}, keys, time.Now().Add(-time.Duration(*SinceSeconds)*time.Second), time.Now())

	if len(metricsList) == 0 {
		return nil
	}

	res := &metrics.PodMetricsList{
		Items: make([]metrics.PodMetrics, 0),
	}

	for _, batch := range metricsList {
		item := metrics.PodMetrics{
			ObjectMeta: metav1.ObjectMeta{
				Name:              pod.Name,
				Namespace:         pod.Namespace,
				CreationTimestamp: metav1.NewTime(time.Now()),
			},
			Timestamp:  metav1.NewTime(batch.Timestamp),
			Window:     metav1.Duration{Duration: time.Minute},
			Containers: make([]metrics.ContainerMetrics, 0),
		}

		for _, container := range pod.Spec.Containers {
			key := core.PodContainerKey(pod.Namespace, pod.Name, container.Name)
			usage := metrics.ResourceList{
				metrics.ResourceName(v1.ResourceCPU.String()): *resource.NewMilliQuantity(
					batch.MetricSets[key].MetricValues[core.MetricCpuUsageRate.MetricDescriptor.Name].IntValue,
					resource.DecimalSI),
				metrics.ResourceName(v1.ResourceMemory.String()): *resource.NewQuantity(
					batch.MetricSets[key].MetricValues[core.MetricMemoryWorkingSet.MetricDescriptor.Name].IntValue,
					resource.BinarySI),
			}
			item.Containers = append(item.Containers, metrics.ContainerMetrics{Name: container.Name, Usage: usage})
		}
		res.Items = append(res.Items, item)
	}

	return res
}

func (m *MetricStorage) getPodMetrics(pod *v1.Pod) *metrics.PodMetrics {
	batch := m.metricSink.GetLatestDataBatch()
	if batch == nil {
		return nil
	}

	res := &metrics.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{
			Name:              pod.Name,
			Namespace:         pod.Namespace,
			CreationTimestamp: metav1.NewTime(time.Now()),
		},
		Timestamp:  metav1.NewTime(batch.Timestamp),
		Window:     metav1.Duration{Duration: time.Minute},
		Containers: make([]metrics.ContainerMetrics, 0),
	}

	for _, c := range pod.Spec.Containers {
		ms, found := batch.MetricSets[core.PodContainerKey(pod.Namespace, pod.Name, c.Name)]
		if !found {
			glog.Infof("No metrics for container %s in pod %s/%s", c.Name, pod.Namespace, pod.Name)
			return nil
		}
		usage, err := util.ParseResourceList(ms)
		if err != nil {
			return nil
		}
		res.Containers = append(res.Containers, metrics.ContainerMetrics{Name: c.Name, Usage: usage})
	}

	return res
}
