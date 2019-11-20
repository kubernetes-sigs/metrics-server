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

package generic

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	coreinf "k8s.io/client-go/informers/core/v1"
	"k8s.io/metrics/pkg/apis/metrics"
	"k8s.io/metrics/pkg/apis/metrics/install"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"sigs.k8s.io/metrics-server/pkg/provider"
	nodemetricsstorage "sigs.k8s.io/metrics-server/pkg/storage/nodemetrics"
	podmetricsstorage "sigs.k8s.io/metrics-server/pkg/storage/podmetrics"
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

// ProviderConfig holds the providers for node and pod metrics
// for serving the resource metrics API.
type ProviderConfig struct {
	Node provider.NodeMetricsProvider
	Pod  provider.PodMetricsProvider
}

// BuildStorage constructs APIGroupInfo the metrics.k8s.io API group using the given providers.
func BuildStorage(providers *ProviderConfig, informers coreinf.Interface) genericapiserver.APIGroupInfo {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(metrics.GroupName, Scheme, metav1.ParameterCodec, Codecs)

	nodemetricsStorage := nodemetricsstorage.NewStorage(metrics.Resource("nodemetrics"), providers.Node, informers.Nodes().Lister())
	podmetricsStorage := podmetricsstorage.NewStorage(metrics.Resource("podmetrics"), providers.Pod, informers.Pods().Lister())
	metricsServerResources := map[string]rest.Storage{
		"nodes": nodemetricsStorage,
		"pods":  podmetricsStorage,
	}
	apiGroupInfo.VersionedResourcesStorageMap[v1beta1.SchemeGroupVersion.Version] = metricsServerResources

	return apiGroupInfo
}

// InstallStorage builds the storage for the metrics.k8s.io API, and then installs it into the given API server.
func InstallStorage(providers *ProviderConfig, informers coreinf.Interface, server *genericapiserver.GenericAPIServer) error {
	info := BuildStorage(providers, informers)
	return server.InstallAPIGroup(&info)
}
