// Copyright 2020 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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
	"k8s.io/component-base/logs"
	logsapi "k8s.io/component-base/logs/api/v1"
	_ "k8s.io/component-base/logs/json/register"

	"sigs.k8s.io/metrics-server/pkg/api"
	generatedopenapi "sigs.k8s.io/metrics-server/pkg/api/generated/openapi"
	"sigs.k8s.io/metrics-server/pkg/server"
)

type Options struct {
	// genericoptions.RecomendedOptions - EtcdOptions
	SecureServing  *genericoptions.SecureServingOptionsWithLoopback
	Authentication *genericoptions.DelegatingAuthenticationOptions
	Authorization  *genericoptions.DelegatingAuthorizationOptions
	Audit          *genericoptions.AuditOptions
	Features       *genericoptions.FeatureOptions
	KubeletClient  *KubeletClientOptions
	Logging        *logs.Options

	MetricResolution time.Duration
	ShowVersion      bool
	Kubeconfig       string

	// DisableHTTP2 indicates that http2 should not be enabled.
	DisableHTTP2 bool

	// Only to be used to for testing
	DisableAuthForTesting bool
}

func (o *Options) Validate() []error {
	errors := o.KubeletClient.Validate()
	errors = append(errors, o.validate()...)
	err := logsapi.ValidateAndApply(o.Logging, nil)
	if err != nil {
		errors = append(errors, err)
	}
	return errors
}

func (o *Options) validate() []error {
	errors := []error{}
	if o.MetricResolution < 10*time.Second {
		errors = append(errors, fmt.Errorf("metric-resolution should be a time duration at least 10s, but value %v provided", o.MetricResolution))
	}
	if o.MetricResolution*9/10 < o.KubeletClient.KubeletRequestTimeout {
		errors = append(errors, fmt.Errorf("metric-resolution should be larger than kubelet-request-timeout, but metric-resolution value %v kubelet-request-timeout value %v provided", o.MetricResolution, o.KubeletClient.KubeletRequestTimeout))
	}
	return errors
}

func (o *Options) Flags() (fs flag.NamedFlagSets) {
	msfs := fs.FlagSet("metrics server")
	msfs.DurationVar(&o.MetricResolution, "metric-resolution", o.MetricResolution, "The resolution at which metrics-server will retain metrics, must set value at least 10s.")
	msfs.BoolVar(&o.ShowVersion, "version", false, "Show version")
	msfs.StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig, "The path to the kubeconfig used to connect to the Kubernetes API server and the Kubelets (defaults to in-cluster config)")
	msfs.BoolVar(&o.DisableHTTP2, "disable-http2", true, "Disable HTTP/2 support")

	o.KubeletClient.AddFlags(fs.FlagSet("kubelet client"))
	o.SecureServing.AddFlags(fs.FlagSet("apiserver secure serving"))
	o.Authentication.AddFlags(fs.FlagSet("apiserver authentication"))
	o.Authorization.AddFlags(fs.FlagSet("apiserver authorization"))
	o.Audit.AddFlags(fs.FlagSet("apiserver audit log"))
	o.Features.AddFlags(fs.FlagSet("features"))
	logsapi.AddFlags(o.Logging, fs.FlagSet("logging"))

	return fs
}

// NewOptions constructs a new set of default options for metrics-server.
func NewOptions() *Options {
	return &Options{
		SecureServing:  genericoptions.NewSecureServingOptions().WithLoopback(),
		Authentication: genericoptions.NewDelegatingAuthenticationOptions(),
		Authorization:  genericoptions.NewDelegatingAuthorizationOptions(),
		Features:       genericoptions.NewFeatureOptions(),
		Audit:          genericoptions.NewAuditOptions(),
		KubeletClient:  NewKubeletClientOptions(),
		Logging:        logs.NewOptions(),

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
		ScrapeTimeout:    o.KubeletClient.KubeletRequestTimeout,
		NodeSelector:     o.KubeletClient.NodeSelector,
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

	// disable HTTP/2 to mitigate CVE-2023-44487 until the Go standard library
	// and golang.org/x/net are fully fixed.
	serverConfig.SecureServing.DisableHTTP2 = o.DisableHTTP2

	if !o.DisableAuthForTesting {
		if err := o.Authentication.ApplyTo(&serverConfig.Authentication, serverConfig.SecureServing, nil); err != nil {
			return nil, err
		}
		if err := o.Authorization.ApplyTo(&serverConfig.Authorization); err != nil {
			return nil, err
		}
	}

	if err := o.Audit.ApplyTo(serverConfig); err != nil {
		return nil, err
	}

	versionGet := version.Get()
	serverConfig.Version = &versionGet
	// enable OpenAPI schemas
	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(generatedopenapi.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(api.Scheme))
	serverConfig.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(generatedopenapi.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(api.Scheme))
	serverConfig.OpenAPIConfig.Info.Title = "Kubernetes metrics-server"
	serverConfig.OpenAPIV3Config.Info.Title = "Kubernetes metrics-server"
	serverConfig.OpenAPIConfig.Info.Version = strings.Split(serverConfig.Version.String(), "-")[0] // TODO(directxman12): remove this once autosetting this doesn't require security definitions
	serverConfig.OpenAPIV3Config.Info.Version = strings.Split(serverConfig.Version.String(), "-")[0]

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
	err = rest.SetKubernetesDefaults(config)
	if err != nil {
		return nil, err
	}
	return config, nil
}
