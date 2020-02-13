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

package summary

import (
	"fmt"

	"k8s.io/client-go/rest"
)

// GetKubeletConfig fetches connection config for connecting to the Kubelet.
func GetKubeletConfig(cfg *rest.Config, port int, insecureTLS bool, completelyInsecure bool) *KubeletClientConfig {
	if completelyInsecure {
		cfg = rest.AnonymousClientConfig(cfg)        // don't use auth to avoid leaking auth details to insecure endpoints
		cfg.TLSClientConfig = rest.TLSClientConfig{} // empty TLS config --> no TLS
	} else if insecureTLS {
		cfg.TLSClientConfig.Insecure = true
		cfg.TLSClientConfig.CAData = nil
		cfg.TLSClientConfig.CAFile = ""
	}
	kubeletConfig := &KubeletClientConfig{
		Port:                         port,
		RESTConfig:                   cfg,
		DeprecatedCompletelyInsecure: completelyInsecure,
	}

	return kubeletConfig
}

// KubeletClientConfig represents configuration for connecting to Kubelets.
type KubeletClientConfig struct {
	Port                         int
	RESTConfig                   *rest.Config
	DeprecatedCompletelyInsecure bool
}

// KubeletClientFor constructs a new KubeletInterface for the given configuration.
func KubeletClientFor(config *KubeletClientConfig) (KubeletInterface, error) {
	transport, err := rest.TransportFor(config.RESTConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to construct transport: %v", err)
	}

	return NewKubeletClient(transport, config.Port, config.DeprecatedCompletelyInsecure)
}
