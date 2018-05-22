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
	"github.com/golang/glog"

	api "github.com/kubernetes-incubator/metrics-server/metrics/api/v1alpha1"
	"github.com/kubernetes-incubator/metrics-server/metrics/options"
	metricsink "github.com/kubernetes-incubator/metrics-server/metrics/sinks/metric"
	nodemetricsstorage "github.com/kubernetes-incubator/metrics-server/metrics/storage/nodemetrics"
	podmetricsstorage "github.com/kubernetes-incubator/metrics-server/metrics/storage/podmetrics"
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/metrics/pkg/apis/metrics"
	"k8s.io/metrics/pkg/apis/metrics/install"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

var (
	groupFactoryRegistry = make(announced.APIGroupFactoryRegistry)
	registry             = registered.NewOrDie("")
	Scheme               = runtime.NewScheme()
	Codecs               = serializer.NewCodecFactory(Scheme)
)

func installMetricsAPIs(s *options.HeapsterRunOptions, g *genericapiserver.GenericAPIServer,
	metricSink *metricsink.MetricSink, nodeLister v1listers.NodeLister, podLister v1listers.PodLister) {
	install.Install(groupFactoryRegistry, registry, Scheme)

	// we need to add the options to empty v1
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Group: "", Version: "v1"})

	unversioned := schema.GroupVersion{Group: "", Version: "v1"}
	Scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
		&api.MetricsOptions{},
	)

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(metrics.GroupName, registry, Scheme, runtime.NewParameterCodec(Scheme), Codecs)
	apiGroupInfo.GroupMeta.GroupVersion = v1beta1.SchemeGroupVersion

	nodemetricsStorage := nodemetricsstorage.NewStorage(metrics.Resource("nodemetrics"), metricSink, nodeLister)
	podmetricsStorage := podmetricsstorage.NewStorage(metrics.Resource("podmetrics"), metricSink, podLister)
	heapsterResources := map[string]rest.Storage{
		"nodes": nodemetricsStorage,
		"pods":  podmetricsStorage,
	}
	apiGroupInfo.VersionedResourcesStorageMap[v1beta1.SchemeGroupVersion.Version] = heapsterResources

	if err := g.InstallAPIGroup(&apiGroupInfo); err != nil {
		glog.Fatalf("Error in registering group versions: %v", err)
	}
}
