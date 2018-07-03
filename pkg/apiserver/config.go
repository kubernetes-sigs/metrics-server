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

package apiserver

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/informers"
	"k8s.io/metrics/pkg/apis/metrics"
	"k8s.io/metrics/pkg/apis/metrics/install"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"github.com/kubernetes-incubator/metrics-server/pkg/provider"
	nodemetricsstorage "github.com/kubernetes-incubator/metrics-server/pkg/storage/nodemetrics"
	podmetricsstorage "github.com/kubernetes-incubator/metrics-server/pkg/storage/podmetrics"
	"github.com/kubernetes-incubator/metrics-server/pkg/version"
)

var (
	Scheme = runtime.NewScheme()
	Codecs = serializer.NewCodecFactory(Scheme)
)

type ProviderConfig struct {
	Node provider.NodeMetricsProvider
	Pod  provider.PodMetricsProvider
}

type Config struct {
	GenericConfig  *genericapiserver.Config
	ProviderConfig ProviderConfig
}

func init() {
	install.Install(Scheme)
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})
}

type completedConfig struct {
	genericapiserver.CompletedConfig
	ProviderConfig *ProviderConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *Config) Complete(informers informers.SharedInformerFactory) completedConfig {
	c.GenericConfig.Version = version.VersionInfo()
	return completedConfig{
		CompletedConfig: c.GenericConfig.Complete(informers),
		ProviderConfig:  &c.ProviderConfig,
	}
}

type MetricsServer struct {
	*genericapiserver.GenericAPIServer
}

// New returns a new instance of MetricsServer from the given config.
func (c completedConfig) New() (*MetricsServer, error) {
	genericServer, err := c.CompletedConfig.New("metrics-server", genericapiserver.NewEmptyDelegate()) // completion is done in Complete, no need for a second time
	if err != nil {
		return nil, err
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(metrics.GroupName, Scheme, metav1.ParameterCodec, Codecs)
	corev1Informers := c.SharedInformerFactory.Core().V1()

	nodemetricsStorage := nodemetricsstorage.NewStorage(metrics.Resource("nodemetrics"), c.ProviderConfig.Node, corev1Informers.Nodes().Lister())
	podmetricsStorage := podmetricsstorage.NewStorage(metrics.Resource("podmetrics"), c.ProviderConfig.Pod, corev1Informers.Pods().Lister())
	metricsServerResources := map[string]rest.Storage{
		"nodes": nodemetricsStorage,
		"pods":  podmetricsStorage,
	}
	apiGroupInfo.VersionedResourcesStorageMap[v1beta1.SchemeGroupVersion.Version] = metricsServerResources

	if err := genericServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return &MetricsServer{
		GenericAPIServer: genericServer,
	}, nil
}
