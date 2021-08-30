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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	corev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/metrics/pkg/apis/metrics"
	"k8s.io/metrics/pkg/apis/metrics/install"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

var (
	// Scheme contains the types needed by the resource metrics API.
	Scheme = runtime.NewScheme()
	// Codecs is a codec factory for serving the resource metrics API.
	Codecs = serializer.NewCodecFactory(Scheme)
)

func init() {
	install.Install(Scheme)
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})
}

// Build constructs APIGroupInfo the metrics.k8s.io API group using the given getters.
func Build(m MetricsGetter, podLister corev1.PodLister, nodeLister corev1.NodeLister) genericapiserver.APIGroupInfo {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(metrics.GroupName, Scheme, metav1.ParameterCodec, Codecs)

	node := newNodeMetrics(metrics.Resource("nodemetrics"), m, nodeLister)
	pod := newPodMetrics(metrics.Resource("podmetrics"), m, podLister)
	metricsServerResources := map[string]rest.Storage{
		"nodemetrics": node,
		"podmetrics":  pod,
	}
	apiGroupInfo.VersionedResourcesStorageMap[v1beta1.SchemeGroupVersion.Version] = metricsServerResources

	return apiGroupInfo
}

// InstallStorage builds the metrics for the metrics.k8s.io API, and then installs it into the given API metrics-server.
func Install(metrics MetricsGetter, podLister corev1.PodLister, nodeLister corev1.NodeLister, server *genericapiserver.GenericAPIServer) error {
	info := Build(metrics, podLister, nodeLister)
	return server.InstallAPIGroup(&info)
}
