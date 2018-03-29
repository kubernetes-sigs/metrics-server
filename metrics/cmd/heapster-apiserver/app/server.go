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

// Package app does all of the work necessary to create a Heapster
// APIServer by binding together the Master Metrics API.
// It can be configured and called directly or via the hyperkube framework.
package app

import (
	"fmt"
	"net"

	"github.com/kubernetes-incubator/metrics-server/metrics/options"
	"github.com/kubernetes-incubator/metrics-server/metrics/provider"
	"k8s.io/apimachinery/pkg/util/wait"
	genericapiserver "k8s.io/apiserver/pkg/server"
	v1listers "k8s.io/client-go/listers/core/v1"
)

const (
	msName = "Metrics Server"
)

type HeapsterAPIServer struct {
	*genericapiserver.GenericAPIServer
}

// Run runs the specified APIServer. This should never exit.
func (h *HeapsterAPIServer) RunServer() error {
	return h.PrepareRun().Run(wait.NeverStop)
}

func NewHeapsterApiServer(s *options.HeapsterRunOptions, nodeProv provider.NodeMetricsProvider,
	podProv provider.PodMetricsProvider, nodeLister v1listers.NodeLister,
	podLister v1listers.PodLister) (*HeapsterAPIServer, error) {

	server, err := newAPIServer(s)
	if err != nil {
		return &HeapsterAPIServer{}, err
	}

	installMetricsAPIs(s, server, nodeProv, podProv, nodeLister, podLister)

	return &HeapsterAPIServer{
		GenericAPIServer: server,
	}, nil
}

func newAPIServer(s *options.HeapsterRunOptions) (*genericapiserver.GenericAPIServer, error) {
	if err := s.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	serverConfig := genericapiserver.NewConfig(Codecs)
	serverConfig.EnableMetrics = true

	if err := s.SecureServing.ApplyTo(serverConfig); err != nil {
		return nil, err
	}

	if !s.DisableAuthForTesting {
		if err := s.Authentication.ApplyTo(serverConfig); err != nil {
			return nil, err
		}
		if err := s.Authorization.ApplyTo(serverConfig); err != nil {
			return nil, err
		}
	}

	serverConfig.SwaggerConfig = genericapiserver.DefaultSwaggerConfig()

	return serverConfig.Complete().New(msName, genericapiserver.EmptyDelegate)
}
