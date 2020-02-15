package options

import (
	"fmt"
	"net"
	"time"

	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"

	apiserver "sigs.k8s.io/metrics-server/pkg/metrics-server"
	summary "sigs.k8s.io/metrics-server/pkg/utils"
)


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

func Flags() {
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
