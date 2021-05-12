// Copyright 2021 The Kubernetes Authors.
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

	"sigs.k8s.io/metrics-server/pkg/scraper/client"

	"github.com/spf13/pflag"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/metrics-server/pkg/utils"
)

type KubeletClientOptions struct {
	KubeletUseNodeStatusPort            bool
	KubeletPort                         int
	InsecureKubeletTLS                  bool
	KubeletPreferredAddressTypes        []string
	KubeletCAFile                       string
	KubeletClientKeyFile                string
	KubeletClientCertFile               string
	DeprecatedCompletelyInsecureKubelet bool
}

func (o *KubeletClientOptions) Validate() []error {
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

func (o *KubeletClientOptions) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&o.InsecureKubeletTLS, "kubelet-insecure-tls", o.InsecureKubeletTLS, "Do not verify CA of serving certificates presented by Kubelets.  For testing purposes only.")
	fs.BoolVar(&o.KubeletUseNodeStatusPort, "kubelet-use-node-status-port", o.KubeletUseNodeStatusPort, "Use the port in the node status. Takes precedence over --kubelet-port flag.")
	fs.IntVar(&o.KubeletPort, "kubelet-port", o.KubeletPort, "The port to use to connect to Kubelets.")
	fs.StringSliceVar(&o.KubeletPreferredAddressTypes, "kubelet-preferred-address-types", o.KubeletPreferredAddressTypes, "The priority of node address types to use when determining which address to use to connect to a particular node")
	fs.StringVar(&o.KubeletCAFile, "kubelet-certificate-authority", "", "Path to the CA to use to validate the Kubelet's serving certificates.")
	fs.StringVar(&o.KubeletClientKeyFile, "kubelet-client-key", "", "Path to a client key file for TLS.")
	fs.StringVar(&o.KubeletClientCertFile, "kubelet-client-certificate", "", "Path to a client cert file for TLS.")
	// MarkDeprecated hides the flag from the help. We don't want that.
	fs.BoolVar(&o.DeprecatedCompletelyInsecureKubelet, "deprecated-kubelet-completely-insecure", o.DeprecatedCompletelyInsecureKubelet, "DEPRECATED: Do not use any encryption, authorization, or authentication when communicating with the Kubelet. This is rarely the right option, since it leaves kubelet communication completely insecure.  If you encounter auth errors, make sure you've enabled token webhook auth on the Kubelet, and if you're in a test cluster with self-signed Kubelet certificates, consider using kubelet-insecure-tls instead.")
}

// NewOptions constructs a new set of default options for metrics-server.
func NewKubeletClientOptions() *KubeletClientOptions {
	o := &KubeletClientOptions{
		KubeletPort:                  10250,
		KubeletPreferredAddressTypes: make([]string, len(utils.DefaultAddressTypePriority)),
	}

	for i, addrType := range utils.DefaultAddressTypePriority {
		o.KubeletPreferredAddressTypes[i] = string(addrType)
	}

	return o
}

func (o KubeletClientOptions) Config(restConfig *rest.Config) *client.KubeletClientConfig {
	config := &client.KubeletClientConfig{
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

func (o KubeletClientOptions) addressResolverConfig() []corev1.NodeAddressType {
	addrPriority := make([]corev1.NodeAddressType, len(o.KubeletPreferredAddressTypes))
	for i, addrType := range o.KubeletPreferredAddressTypes {
		addrPriority[i] = corev1.NodeAddressType(addrType)
	}
	return addrPriority
}
