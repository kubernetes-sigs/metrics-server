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

package app

import (
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/healthz"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"sigs.k8s.io/metrics-server/pkg/apiserver"
	genericmetrics "sigs.k8s.io/metrics-server/pkg/apiserver/generic"
	"sigs.k8s.io/metrics-server/pkg/manager"
	"sigs.k8s.io/metrics-server/pkg/provider/sink"
	"sigs.k8s.io/metrics-server/pkg/sources"
	"sigs.k8s.io/metrics-server/pkg/sources/summary"
	"sigs.k8s.io/metrics-server/pkg/version"
)

// NewCommandStartMetricsServer provides a CLI handler for the metrics server entrypoint
func NewCommandStartMetricsServer(out, errOut io.Writer, stopCh <-chan struct{}) *cobra.Command {
	o := NewMetricsServerOptions()

	cmd := &cobra.Command{
		Short: "Launch metrics-server",
		Long:  "Launch metrics-server",
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Run(stopCh); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.DurationVar(&o.MetricResolution, "metric-resolution", o.MetricResolution, "The resolution at which metrics-server will retain metrics.")

	flags.BoolVar(&o.InsecureKubeletTLS, "kubelet-insecure-tls", o.InsecureKubeletTLS, "Do not verify CA of serving certificates presented by Kubelets.  For testing purposes only.")
	flags.BoolVar(&o.DeprecatedCompletelyInsecureKubelet, "deprecated-kubelet-completely-insecure", o.DeprecatedCompletelyInsecureKubelet, "Do not use any encryption, authorization, or authentication when communicating with the Kubelet.")
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

	return cmd
}

type MetricsServerOptions struct {
	// genericoptions.ReccomendedOptions - EtcdOptions
	SecureServing  *genericoptions.SecureServingOptionsWithLoopback
	Authentication *genericoptions.DelegatingAuthenticationOptions
	Authorization  *genericoptions.DelegatingAuthorizationOptions
	Features       *genericoptions.FeatureOptions

	Kubeconfig string

	// Only to be used to for testing
	DisableAuthForTesting bool

	MetricResolution time.Duration

	KubeletPort                  int
	InsecureKubeletTLS           bool
	KubeletPreferredAddressTypes []string
	KubeletCAFile                string

	ShowVersion bool

	DeprecatedCompletelyInsecureKubelet bool
}

// NewMetricsServerOptions constructs a new set of default options for metrics-server.
func NewMetricsServerOptions() *MetricsServerOptions {
	o := &MetricsServerOptions{
		SecureServing:  genericoptions.NewSecureServingOptions().WithLoopback(),
		Authentication: genericoptions.NewDelegatingAuthenticationOptions(),
		Authorization:  genericoptions.NewDelegatingAuthorizationOptions(),
		Features:       genericoptions.NewFeatureOptions(),

		MetricResolution:             60 * time.Second,
		KubeletPort:                  10250,
		KubeletPreferredAddressTypes: make([]string, len(summary.DefaultAddressTypePriority)),
	}

	for i, addrType := range summary.DefaultAddressTypePriority {
		o.KubeletPreferredAddressTypes[i] = string(addrType)
	}

	return o
}

func (o MetricsServerOptions) Config() (*apiserver.Config, error) {
	if err := o.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	serverConfig := genericapiserver.NewConfig(genericmetrics.Codecs)
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

	return &apiserver.Config{
		GenericConfig:  serverConfig,
		ProviderConfig: genericmetrics.ProviderConfig{},
	}, nil
}

func (o MetricsServerOptions) Run(stopCh <-chan struct{}) error {
	if o.ShowVersion {
		fmt.Println(version.VersionInfo())
		os.Exit(0)
	}

	// grab the config for the API server
	config, err := o.Config()
	if err != nil {
		return err
	}
	config.GenericConfig.EnableMetrics = true

	// set up the client config
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

	// complete the config to get an API server
	server, err := config.Complete(informerFactory).New()
	if err != nil {
		return err
	}

	// add health checks
	err = server.AddHealthChecks(healthz.NamedCheck("healthz", mgr.CheckHealth))
	if err != nil {
		return err
	}

	// run everything (the apiserver runs the shared informer factory for us)
	mgr.RunUntil(stopCh)
	return server.GenericAPIServer.PrepareRun().Run(stopCh)
}
