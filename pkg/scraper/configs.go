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

package scraper

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/metrics-server/pkg/utils"
)

// KubeletClientConfig represents configuration for connecting to Kubelets.
type KubeletClientConfig struct {
	Client              rest.Config
	AddressTypePriority []corev1.NodeAddressType
	Scheme              string
	DefaultPort         int
	UseNodeStatusPort   bool
}

// Complete constructs a new kubeletCOnfig for the given configuration.
func (config KubeletClientConfig) Complete() (*kubeletClient, error) {
	transport, err := rest.TransportFor(&config.Client)
	if err != nil {
		return nil, fmt.Errorf("unable to construct transport: %v", err)
	}

	c := &http.Client{
		Transport: transport,
	}
	return &kubeletClient{
		addrResolver:      utils.NewPriorityNodeAddressResolver(config.AddressTypePriority),
		defaultPort:       config.DefaultPort,
		client:            c,
		scheme:            config.Scheme,
		useNodeStatusPort: config.UseNodeStatusPort,
		buffers: sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}, nil
}
