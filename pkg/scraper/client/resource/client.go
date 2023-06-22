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

package resource

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/metrics-server/pkg/scraper/client"
	"sigs.k8s.io/metrics-server/pkg/storage"
	"sigs.k8s.io/metrics-server/pkg/utils"
)

const (
	// AnnotationResourceMetricsPath is the annotation used to specify the path to the resource metrics endpoint.
	AnnotationResourceMetricsPath = "metrics.k8s.io/resource-metrics-path"
)

type kubeletClient struct {
	defaultPort       int
	useNodeStatusPort bool
	client            *http.Client
	scheme            string
	addrResolver      utils.NodeAddressResolver
	buffers           sync.Pool
}

var _ client.KubeletMetricsGetter = (*kubeletClient)(nil)

func NewForConfig(config *client.KubeletClientConfig) (*kubeletClient, error) {
	transport, err := rest.TransportFor(&config.Client)
	if err != nil {
		return nil, fmt.Errorf("unable to construct transport: %v", err)
	}

	c := &http.Client{
		Transport: transport,
		Timeout:   config.Client.Timeout,
	}
	return newClient(c, utils.NewPriorityNodeAddressResolver(config.AddressTypePriority), config.DefaultPort, config.Scheme, config.UseNodeStatusPort), nil
}

func newClient(c *http.Client, resolver utils.NodeAddressResolver, defaultPort int, scheme string, useNodeStatusPort bool) *kubeletClient {
	return &kubeletClient{
		addrResolver:      resolver,
		defaultPort:       defaultPort,
		client:            c,
		scheme:            scheme,
		useNodeStatusPort: useNodeStatusPort,
		buffers: sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 10e3)
				return &buf
			},
		},
	}
}

// GetMetrics implements client.KubeletMetricsGetter
func (kc *kubeletClient) GetMetrics(ctx context.Context, node *corev1.Node) (*storage.MetricsBatch, error) {
	port := kc.defaultPort
	path := "/metrics/resource"
	nodeStatusPort := int(node.Status.DaemonEndpoints.KubeletEndpoint.Port)
	if kc.useNodeStatusPort && nodeStatusPort != 0 {
		port = nodeStatusPort
	}
	if node.Annotations != nil &&
		node.Annotations[AnnotationResourceMetricsPath] != "" {
		path = node.Annotations[AnnotationResourceMetricsPath]
	}
	addr, err := kc.addrResolver.NodeAddress(node)
	if err != nil {
		return nil, err
	}
	url := url.URL{
		Scheme: kc.scheme,
		Host:   net.JoinHostPort(addr, strconv.Itoa(port)),
		Path:   path,
	}
	return kc.getMetrics(ctx, url.String(), node.Name)
}

func (kc *kubeletClient) getMetrics(ctx context.Context, url, nodeName string) (*storage.MetricsBatch, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	requestTime := time.Now()
	response, err := kc.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed, status: %q", response.Status)
	}
	bp := kc.buffers.Get().(*[]byte)
	b := *bp
	defer func() {
		*bp = b
		kc.buffers.Put(bp)
	}()
	buf := bytes.NewBuffer(b)
	buf.Reset()
	_, err = io.Copy(buf, response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body - %v", err)
	}
	b = buf.Bytes()
	ms, err := decodeBatch(b, requestTime, nodeName)
	if err != nil {
		return nil, err
	}
	return ms, nil
}
