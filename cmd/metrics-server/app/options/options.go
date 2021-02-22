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

	corev1 "k8s.io/api/core/v1"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/cli/flag"

	"sigs.k8s.io/metrics-server/pkg/api"
	generatedopenapi "sigs.k8s.io/metrics-server/pkg/api/generated/openapi"
	"sigs.k8s.io/metrics-server/pkg/scraper"
	"sigs.k8s.io/metrics-server/pkg/server"
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
	KubeletClientKeyFile         string
	KubeletClientCertFile        string

	ShowVersion bool

	DeprecatedCompletelyInsecureKubelet bool
}

func (o *Options) Validate() []error {
	errors := []error{}
	if (o.KubeletCAFile != "") && o.InsecureKubeletTLS {
		errors = append(errors, fmt.Errorf("Cannot use both --kubelet-certificate-authority and --kubelet-insecure-tls"))
	}

	if (o.KubeletClientKeyFile != "") != (o.KubeletClientCertFile != "") {
		errors = append(errors, fmt.Errorf("Need both --kubelet-client-key and --kubelet-client-certificate"))
	}

	if (o.KubeletClientKeyFile != "") && o.DeprecatedCompletelyInsecureKubelet {
		errors = append(errors, fmt.Errorf("Cannot use both --kubelet-client-key and --deprecated-kubelet-completely-insecure"))
	}

	if (o.KubeletClientCertFile != "") && o.DeprecatedCompletelyInsecureKubelet {
		errors = append(errors, fmt.Errorf("Cannot use both --kubelet-client-certificate and --deprecated-kubelet-completely-insecure"))
	}

	if o.InsecureKubeletTLS && o.DeprecatedCompletelyInsecureKubelet {
		errors = append(errors, fmt.Errorf("Cannot use both --kubelet-insecure-tls and --deprecated-kubelet-completely-insecure"))
	}
	if (o.KubeletCAFile != "") && o.DeprecatedCompletelyInsecureKubelet {
		errors = append(errors, fmt.Errorf("Cannot use both --kubelet-certificate-authority and --deprecated-kubelet-completely-insecure"))
	}
	return errors
}

func (o *Options) Flags() (fs flag.NamedFlagSets) {
	msfs := fs.FlagSet("metrics server")
	msfs.DurationVar(&o.MetricResolution, "metric-resolution", o.MetricResolution, "The resolution at which metrics-server will retain metrics.")
	msfs.BoolVar(&o.ShowVersion, "version", false, "Show version")
	msfs.StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig, "The path to the kubeconfig used to connect to the Kubernetes API server and the Kubelets (defaults to in-cluster config)")

	kfs := fs.FlagSet("kubelet client")
	kfs.BoolVar(&o.InsecureKubeletTLS, "kubelet-insecure-tls", o.InsecureKubeletTLS, "Do not verify CA of serving certificates presented by Kubelets.  For testing purposes only.")
	kfs.BoolVar(&o.KubeletUseNodeStatusPort, "kubelet-use-node-status-port", o.KubeletUseNodeStatusPort, "Use the port in the node status. Takes precedence over --kubelet-port flag.")
	kfs.IntVar(&o.KubeletPort, "kubelet-port", o.KubeletPort, "The port to use to connect to Kubelets.")
	kfs.StringSliceVar(&o.KubeletPreferredAddressTypes, "kubelet-preferred-address-types", o.KubeletPreferredAddressTypes, "The priority of node address types to use when determining which address to use to connect to a particular node")
	kfs.StringVar(&o.KubeletCAFile, "kubelet-certificate-authority", "", "Path to the CA to use to validate the Kubelet's serving certificates.")
	kfs.StringVar(&o.KubeletClientKeyFile, "kubelet-client-key", "", "Path to a client key file for TLS.")
	kfs.StringVar(&o.KubeletClientCertFile, "kubelet-client-certificate", "", "Path to a client cert file for TLS.")
	// MarkDeprecated hides the flag from the help. We don't want that.
	kfs.BoolVar(&o.DeprecatedCompletelyInsecureKubelet, "deprecated-kubelet-completely-insecure", o.DeprecatedCompletelyInsecureKubelet, "DEPRECATED: Do not use any encryption, authorization, or authentication when communicating with the Kubelet. This is rarely the right option, since it leaves kubelet communication completely insecure.  If you encounter auth errors, make sure you've enabled token webhook auth on the Kubelet, and if you're in a test cluster with self-signed Kubelet certificates, consider using kubelet-insecure-tls instead.")

	o.SecureServing.AddFlags(fs.FlagSet("apiserver secure serving"))
	o.Authentication.AddFlags(fs.FlagSet("apiserver authentication"))
	o.Authorization.AddFlags(fs.FlagSet("apiserver authorization"))
	o.Features.AddFlags(fs.FlagSet("features"))

	return fs
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
		Kubelet:          o.kubeletConfig(restConfig),
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
	return config, err
}

func (o Options) kubeletConfig(restConfig *rest.Config) *scraper.KubeletClientConfig {
	config := &scraper.KubeletClientConfig{
		Scheme:              "https",
		DefaultPort:         o.KubeletPort,
		AddressTypePriority: o.addressResolverConfig(),
		UseNodeStatusPort:   o.KubeletUseNodeStatusPort,
		Client:              *rest.CopyConfig(restConfig),
	}
	if o.DeprecatedCompletelyInsecureKubelet {
		config.Scheme = "http"
		config.Client = *rest.AnonymousClientConfig(&config.Client) // don't use auth to avoid leaking auth details to insecure endpoints
		config.Client.TLSClientConfig = rest.TLSClientConfig{}      // empty TLS config --> no TLS
	}
	if o.InsecureKubeletTLS {
		config.Client.TLSClientConfig.Insecure = true
		config.Client.TLSClientConfig.CAData = nil
		config.Client.TLSClientConfig.CAFile = ""
	}
	if len(o.KubeletCAFile) > 0 {
		config.Client.TLSClientConfig.CAFile = o.KubeletCAFile
		config.Client.TLSClientConfig.CAData = nil
	}
	if len(o.KubeletClientCertFile) > 0 {
		config.Client.TLSClientConfig.CertFile = o.KubeletClientCertFile
		config.Client.TLSClientConfig.CertData = nil
	}
	if len(o.KubeletClientKeyFile) > 0 {
		config.Client.TLSClientConfig.KeyFile = o.KubeletClientKeyFile
		config.Client.TLSClientConfig.KeyData = nil
	}
	return config
}

func (o Options) addressResolverConfig() []corev1.NodeAddressType {
	addrPriority := make([]corev1.NodeAddressType, len(o.KubeletPreferredAddressTypes))
	for i, addrType := range o.KubeletPreferredAddressTypes {
		addrPriority[i] = corev1.NodeAddressType(addrType)
	}
	return addrPriority
}
