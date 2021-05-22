// Copyright 2020 The Kubernetes Authors.
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
package options

import (
	"fmt"
	"net"
	"strings"
	"time"

	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/client-go/pkg/version"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/cli/flag"

	"sigs.k8s.io/metrics-server/pkg/api"
	generatedopenapi "sigs.k8s.io/metrics-server/pkg/api/generated/openapi"
	"sigs.k8s.io/metrics-server/pkg/server"
)

type Options struct {
	// genericoptions.ReccomendedOptions - EtcdOptions
	SecureServing  *genericoptions.SecureServingOptionsWithLoopback
	Authentication *genericoptions.DelegatingAuthenticationOptions
	Authorization  *genericoptions.DelegatingAuthorizationOptions
	Features       *genericoptions.FeatureOptions
	KubeletClient  *KubeletClientOptions

	MetricResolution time.Duration
	ShowVersion      bool
	Kubeconfig       string

	// Only to be used to for testing
	DisableAuthForTesting bool
}

func (o *Options) Validate() []error {
	errors := o.KubeletClient.Validate()
	if o.MetricResolution < 10*time.Second {
		errors = append(errors, fmt.Errorf("Metric-resolution should be a time duration at least 10s, but value %v provided", o.MetricResolution))
	}
	return errors
}

func (o *Options) Flags() (fs flag.NamedFlagSets) {
	msfs := fs.FlagSet("metrics server")
	msfs.DurationVar(&o.MetricResolution, "metric-resolution", o.MetricResolution, "The resolution at which metrics-server will retain metrics, must set value at least 10s.")
	msfs.BoolVar(&o.ShowVersion, "version", false, "Show version")
	msfs.StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig, "The path to the kubeconfig used to connect to the Kubernetes API server and the Kubelets (defaults to in-cluster config)")

	o.KubeletClient.AddFlags(fs.FlagSet("kubelet client"))
	o.SecureServing.AddFlags(fs.FlagSet("apiserver secure serving"))
	o.Authentication.AddFlags(fs.FlagSet("apiserver authentication"))
	o.Authorization.AddFlags(fs.FlagSet("apiserver authorization"))
	o.Features.AddFlags(fs.FlagSet("features"))

	return fs
}

// NewOptions constructs a new set of default options for metrics-server.
func NewOptions() *Options {
	return &Options{
		SecureServing:  genericoptions.NewSecureServingOptions().WithLoopback(),
		Authentication: genericoptions.NewDelegatingAuthenticationOptions(),
		Authorization:  genericoptions.NewDelegatingAuthorizationOptions(),
		Features:       genericoptions.NewFeatureOptions(),
		KubeletClient:  NewKubeletClientOptions(),

		MetricResolution: 60 * time.Second,
	}
}

func (o Options) ServerConfig() (*server.Config, error) {
	apiserver, err := o.ApiserverConfig()
	if err != nil {
		return nil, err
	}
	restConfig, err := o.restConfig()
	if err != nil {
		return nil, err
	}
	return &server.Config{
		Apiserver:        apiserver,
		Rest:             restConfig,
		Kubelet:          o.KubeletClient.Config(restConfig),
		MetricResolution: o.MetricResolution,
		ScrapeTimeout:    time.Duration(float64(o.MetricResolution) * 0.90), // scrape timeout is 90% of the scrape interval
	}, nil
}

func (o Options) ApiserverConfig() (*genericapiserver.Config, error) {
	if err := o.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	serverConfig := genericapiserver.NewConfig(api.Codecs)
	if err := o.SecureServing.ApplyTo(&serverConfig.SecureServing, &serverConfig.LoopbackClientConfig); err != nil {
		return nil, err
	}

	if !o.DisableAuthForTesting {
		if err := o.Authentication.ApplyTo(&serverConfig.Authentication, serverConfig.SecureServing, nil); err != nil {
			return nil, err
		}
		if err := o.Authorization.ApplyTo(&serverConfig.Authorization); err != nil {
			return nil, err
		}
	}
	versionGet := version.Get()
	serverConfig.Version = &versionGet
	// enable OpenAPI schemas
	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(generatedopenapi.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(api.Scheme))
	serverConfig.OpenAPIConfig.Info.Title = "Kubernetes metrics-server"
	serverConfig.OpenAPIConfig.Info.Version = strings.Split(serverConfig.Version.String(), "-")[0] // TODO(directxman12): remove this once autosetting this doesn't require security definitions

	return serverConfig, nil
}

func (o Options) restConfig() (*rest.Config, error) {
	var config *rest.Config
	var err error
	if len(o.Kubeconfig) > 0 {
		loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: o.Kubeconfig}
		loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

		config, err = loader.ClientConfig()
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("unable to construct lister client config: %v", err)
	}
	// Use protobufs for communication with apiserver
	config.ContentType = "application/vnd.kubernetes.protobuf"
	rest.SetKubernetesDefaults(config)
	return config, err
}
