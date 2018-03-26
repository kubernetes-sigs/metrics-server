// Copyright 2014 Google Inc. All Rights Reserved.
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

package summary

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/golang/glog"
	kube_config "github.com/kubernetes-incubator/metrics-server/common/kubernetes"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	kube_client "k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
)

const (
	APIVersion = "v1"

	defaultKubeletPort        = 10255
	defaultKubeletHttps       = false
	defaultUseServiceAccount  = false
	defaultServiceAccountFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	defaultInClusterConfig    = true
)

func GetKubeConfigs(uri *url.URL) (*kube_client.Config, *KubeletClientConfig, error) {

	kubeConfig, err := kube_config.GetKubeClientConfig(uri)
	if err != nil {
		return nil, nil, err
	}
	opts := uri.Query()

	kubeletPort := defaultKubeletPort
	if len(opts["kubeletPort"]) >= 1 {
		kubeletPort, err = strconv.Atoi(opts["kubeletPort"][0])
		if err != nil {
			return nil, nil, err
		}
	}

	kubeletHttps := defaultKubeletHttps
	if len(opts["kubeletHttps"]) >= 1 {
		kubeletHttps, err = strconv.ParseBool(opts["kubeletHttps"][0])
		if err != nil {
			return nil, nil, err
		}
	}

	glog.Infof("Using Kubernetes client with master %q and version %+v\n", kubeConfig.Host, kubeConfig.GroupVersion)
	glog.Infof("Using kubelet port %d", kubeletPort)

	kubeletConfig := &KubeletClientConfig{
		Port:            uint(kubeletPort),
		EnableHttps:     kubeletHttps,
		TLSClientConfig: kubeConfig.TLSClientConfig,
		BearerToken:     kubeConfig.BearerToken,
	}

	return kubeConfig, kubeletConfig, nil
}

type KubeletClientConfig struct {
	// Default port - used if no information about Kubelet port can be found in Node.NodeStatus.DaemonEndpoints.
	Port         uint
	ReadOnlyPort uint
	EnableHttps  bool

	// PreferredAddressTypes - used to select an address from Node.NodeStatus.Addresses
	PreferredAddressTypes []string

	// TLSClientConfig contains settings to enable transport layer security
	restclient.TLSClientConfig

	// Server requires Bearer authentication
	BearerToken string

	// HTTPTimeout is used by the client to timeout http requests to Kubelet.
	HTTPTimeout time.Duration

	// Dial is a custom dialer used for the client
	Dial utilnet.DialFunc
}

func MakeTransport(config *KubeletClientConfig) (http.RoundTripper, error) {
	tlsConfig, err := transport.TLSConfigFor(config.transportConfig())
	if err != nil {
		return nil, err
	}

	rt := http.DefaultTransport
	if config.Dial != nil || tlsConfig != nil {
		rt = utilnet.SetOldTransportDefaults(&http.Transport{
			Dial:            config.Dial,
			TLSClientConfig: tlsConfig,
		})
	}

	return transport.HTTPWrappersForConfig(config.transportConfig(), rt)
}

// transportConfig converts a client config to an appropriate transport config.
func (c *KubeletClientConfig) transportConfig() *transport.Config {
	cfg := &transport.Config{
		TLS: transport.TLSConfig{
			CAFile:   c.CAFile,
			CAData:   c.CAData,
			CertFile: c.CertFile,
			CertData: c.CertData,
			KeyFile:  c.KeyFile,
			KeyData:  c.KeyData,
		},
	}
	if c.EnableHttps {
		cfg.BearerToken = c.BearerToken
		if !cfg.HasCA() {
			cfg.TLS.Insecure = true
		}
	}
	return cfg
}
