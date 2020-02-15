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
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	summary "sigs.k8s.io/metrics-server/pkg/scraper"
	sink "sigs.k8s.io/metrics-server/pkg/storage"

	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/informers"

	"sigs.k8s.io/metrics-server/pkg/apiserver/generic"
	generatedopenapi "sigs.k8s.io/metrics-server/pkg/generated/openapi"
	"sigs.k8s.io/metrics-server/pkg/version"
)

// Config contains configuration for launching an instance of metrics-server.
type Config struct {
	GenericConfig  *genericapiserver.Config
	ProviderConfig generic.ProviderConfig
}

type completedConfig struct {
	genericapiserver.CompletedConfig
	ProviderConfig *generic.ProviderConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *Config) Complete(informers informers.SharedInformerFactory) completedConfig {
	c.GenericConfig.Version = version.VersionInfo()

	// enable OpenAPI schemas
	c.GenericConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(generatedopenapi.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(generic.Scheme))
	c.GenericConfig.OpenAPIConfig.Info.Title = "Kubernetes metrics-server"
	c.GenericConfig.OpenAPIConfig.Info.Version = strings.Split(c.GenericConfig.Version.String(), "-")[0] // TODO(directxman12): remove this once autosetting this doesn't require security definitions

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
	var clientConfig *rest.Config
	if len(o.Kubeconfig) > 0 {
		loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: o.Kubeconfig}
		loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

		clientConfig, err = loader.ClientConfig()
	} else {
		clientConfig, err = rest.InClusterConfig()
	}
	if err != nil {
		return fmt.Errorf("unable to construct lister client config: %v", err)
	}
	// Use protobufs for communication with apiserver
	clientConfig.ContentType = "application/vnd.kubernetes.protobuf"

	// set up the informers
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return fmt.Errorf("unable to construct lister client: %v", err)
	}
	// we should never need to resync, since we're not worried about missing events,
	// and resync is actually for regular interval-based reconciliation these days,
	// so set the default resync interval to 0
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)

	// set up the source manager
	kubeletRestCfg := rest.CopyConfig(clientConfig)
	if len(o.KubeletCAFile) > 0 {
		kubeletRestCfg.TLSClientConfig.CAFile = o.KubeletCAFile
		kubeletRestCfg.TLSClientConfig.CAData = nil
	}
	kubeletConfig := summary.GetKubeletConfig(kubeletRestCfg, o.KubeletPort, o.InsecureKubeletTLS, o.DeprecatedCompletelyInsecureKubelet)
	kubeletClient, err := summary.KubeletClientFor(kubeletConfig)
	if err != nil {
		return fmt.Errorf("unable to construct a client to connect to the kubelets: %v", err)
	}

	// set up an address resolver according to the user's priorities
	addrPriority := make([]corev1.NodeAddressType, len(o.KubeletPreferredAddressTypes))
	for i, addrType := range o.KubeletPreferredAddressTypes {
		addrPriority[i] = corev1.NodeAddressType(addrType)
	}
	addrResolver := summary.NewPriorityNodeAddressResolver(addrPriority)

	sourceProvider := summary.NewSummaryProvider(informerFactory.Core().V1().Nodes().Lister(), kubeletClient, addrResolver)
	scrapeTimeout := time.Duration(float64(o.MetricResolution) * 0.90) // scrape timeout is 90% of the scrape interval
	sources.RegisterDurationMetrics(scrapeTimeout)
	sourceManager := sources.NewSourceManager(sourceProvider, scrapeTimeout)

	// set up the in-memory sink and provider
	metricSink, metricsProvider := sink.NewSinkProvider()

	// set up the general manager
	manager.RegisterDurationMetrics(o.MetricResolution)
	mgr := manager.NewManager(sourceManager, metricSink, o.MetricResolution)

	// inject the providers into the config
	config.ProviderConfig.Node = metricsProvider
	config.ProviderConfig.Pod = metricsProvider
	genericServer, err := c.CompletedConfig.New("metrics-server", genericapiserver.NewEmptyDelegate()) // completion is done in Complete, no need for a second time
	if err != nil {
		return nil, err
	}

	if err := generic.InstallStorage(c.ProviderConfig, c.SharedInformerFactory.Core().V1(), genericServer); err != nil {
		return nil, err
	}

	return &MetricsServer{
		GenericAPIServer: genericServer,
	}, nil
}
