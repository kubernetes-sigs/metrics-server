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

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"sigs.k8s.io/metrics-server/pkg/api"
	generatedopenapi "sigs.k8s.io/metrics-server/pkg/api/generated/openapi"
	metric_server "sigs.k8s.io/metrics-server/pkg/metrics-server"
	"sigs.k8s.io/metrics-server/pkg/scraper"
	"sigs.k8s.io/metrics-server/pkg/utils"
	"sigs.k8s.io/metrics-server/pkg/version"
)

type Options struct {
	// genericoptions.ReccomendedOptions - EtcdOptions
	SecureServing  *genericoptions.SecureServingOptionsWithLoopback
	Authentication *genericoptions.DelegatingAuthenticationOptions
	Authorization  *genericoptions.DelegatingAuthorizationOptions
	Features       *genericoptions.FeatureOptions

	Kubeconfig string

	// Only to be used to for testing
	DisableAuthForTesting bool

	MetricResolution time.Duration

	KubeletUseNodeStatusPort     bool
	KubeletPort                  int
	InsecureKubeletTLS           bool
	KubeletPreferredAddressTypes []string
	KubeletCAFile                string

	ShowVersion bool

	DeprecatedCompletelyInsecureKubelet bool
}

func (o *Options) Flags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.DurationVar(&o.MetricResolution, "metric-resolution", o.MetricResolution, "The resolution at which metrics-server will retain metrics.")

	flags.BoolVar(&o.InsecureKubeletTLS, "kubelet-insecure-tls", o.InsecureKubeletTLS, "Do not verify CA of serving certificates presented by Kubelets.  For testing purposes only.")
	flags.BoolVar(&o.DeprecatedCompletelyInsecureKubelet, "deprecated-kubelet-completely-insecure", o.DeprecatedCompletelyInsecureKubelet, "Do not use any encryption, authorization, or authentication when communicating with the Kubelet.")
	flags.BoolVar(&o.KubeletUseNodeStatusPort, "kubelet-use-node-status-port", o.KubeletUseNodeStatusPort, "Use the port in the node status. Takes precedence over --kubelet-port flag.")
	flags.IntVar(&o.KubeletPort, "kubelet-port", o.KubeletPort, "The port to use to connect to Kubelets.")
	flags.StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig, "The path to the kubeconfig used to connect to the Kubernetes API server and the Kubelets (defaults to in-cluster config)")
	flags.StringSliceVar(&o.KubeletPreferredAddressTypes, "kubelet-preferred-address-types", o.KubeletPreferredAddressTypes, "The priority of node address types to use when determining which address to use to connect to a particular node")
	flags.StringVar(&o.KubeletCAFile, "kubelet-certificate-authority", "", "Path to the CA to use to validate the Kubelet's serving certificates.")

	flags.BoolVar(&o.ShowVersion, "version", false, "Show version")

	flags.MarkDeprecated("deprecated-kubelet-completely-insecure", "This is rarely the right option, since it leaves kubelet communication completely insecure.  If you encounter auth errors, make sure you've enabled token webhook auth on the Kubelet, and if you're in a test cluster with self-signed Kubelet certificates, consider using kubelet-insecure-tls instead.")

	o.SecureServing.AddFlags(flags)
	o.Authentication.AddFlags(flags)
	o.Authorization.AddFlags(flags)
	o.Features.AddFlags(flags)
}

// NewOptions constructs a new set of default options for metrics-server.
func NewOptions() *Options {
	o := &Options{
		SecureServing:  genericoptions.NewSecureServingOptions().WithLoopback(),
		Authentication: genericoptions.NewDelegatingAuthenticationOptions(),
		Authorization:  genericoptions.NewDelegatingAuthorizationOptions(),
		Features:       genericoptions.NewFeatureOptions(),

		MetricResolution:             60 * time.Second,
		KubeletPort:                  10250,
		KubeletPreferredAddressTypes: make([]string, len(utils.DefaultAddressTypePriority)),
	}

	for i, addrType := range utils.DefaultAddressTypePriority {
		o.KubeletPreferredAddressTypes[i] = string(addrType)
	}

	return o
}

func (o Options) MetricsServerConfig() (*metric_server.Config, error) {
	apiserver, err := o.ApiserverConfig()
	if err != nil {
		return nil, err
	}
	restConfig, err := o.restConfig()
	if err != nil {
		return nil, err
	}
	kubelet := o.kubeletConfig(restConfig)
	addressResolver := o.addressResolverConfig()
	return &metric_server.Config{
		Apiserver:        apiserver,
		Rest:             restConfig,
		Kubelet:          kubelet,
		AddresResolver:   addressResolver,
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
	serverConfig.Version = version.VersionInfo()
	// enable OpenAPI schemas
	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(generatedopenapi.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(api.Scheme))
	serverConfig.OpenAPIConfig.Info.Title = "Kubernetes metrics-server"
	serverConfig.OpenAPIConfig.Info.Version = strings.Split(serverConfig.Version.String(), "-")[0] // TODO(directxman12): remove this once autosetting this doesn't require security definitions

	return serverConfig, nil
}

func (o Options) restConfig() (*rest.Config, error) {
	var clientConfig *rest.Config
	var err error
	if len(o.Kubeconfig) > 0 {
		loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: o.Kubeconfig}
		loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

		clientConfig, err = loader.ClientConfig()
	} else {
		clientConfig, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("unable to construct lister client config: %v", err)
	}
	return clientConfig, err
}

func (o Options) kubeletConfig(restConfig *rest.Config) *scraper.KubeletClientConfig {
	kubeletRestCfg := rest.CopyConfig(restConfig)
	if len(o.KubeletCAFile) > 0 {
		kubeletRestCfg.TLSClientConfig.CAFile = o.KubeletCAFile
		kubeletRestCfg.TLSClientConfig.CAData = nil
	}
	return scraper.GetKubeletConfig(kubeletRestCfg, o.KubeletPort, o.KubeletUseNodeStatusPort, o.InsecureKubeletTLS, o.DeprecatedCompletelyInsecureKubelet)
}

func (o Options) addressResolverConfig() []corev1.NodeAddressType {
	addrPriority := make([]corev1.NodeAddressType, len(o.KubeletPreferredAddressTypes))
	for i, addrType := range o.KubeletPreferredAddressTypes {
		addrPriority[i] = corev1.NodeAddressType(addrType)
	}
	return addrPriority
}
