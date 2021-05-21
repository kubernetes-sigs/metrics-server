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
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/metrics-server/pkg/scraper/client"

	"sigs.k8s.io/metrics-server/pkg/storage"

	"github.com/mailru/easyjson"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/metrics-server/pkg/utils"
)

func NewClient(config client.KubeletClientConfig) (*kubeletClient, error) {
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

type kubeletClient struct {
	defaultPort       int
	useNodeStatusPort bool
	client            *http.Client
	scheme            string
	addrResolver      utils.NodeAddressResolver
	buffers           sync.Pool
}

var _ client.KubeletMetricsInterface = (*kubeletClient)(nil)

func (kc *kubeletClient) makeRequestAndGetValue(client *http.Client, req *http.Request, value easyjson.Unmarshaler) error {
	// TODO(directxman12): support validating certs by hostname
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	b := kc.getBuffer()
	defer kc.returnBuffer(b)
	_, err = io.Copy(b, response.Body)
	if err != nil {
		return err
	}
	body := b.Bytes()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %q: bad status code %q", req.URL, response.Status)
	}

	err = easyjson.Unmarshal(body, value)
	if err != nil {
		return fmt.Errorf("GET %q: failed to parse output: %w", req.URL, err)
	}
	return nil
}

func (kc *kubeletClient) GetMetrics(ctx context.Context, node *corev1.Node) (*storage.MetricsBatch, error) {
	port := kc.defaultPort
	nodeStatusPort := int(node.Status.DaemonEndpoints.KubeletEndpoint.Port)
	if kc.useNodeStatusPort && nodeStatusPort != 0 {
		port = nodeStatusPort
	}
	addr, err := kc.addrResolver.NodeAddress(node)
	if err != nil {
		return nil, err
	}
	url := url.URL{
		Scheme:   kc.scheme,
		Host:     net.JoinHostPort(addr, strconv.Itoa(port)),
		Path:     "/stats/summary",
		RawQuery: "only_cpu_and_memory=true",
	}

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, err
	}
	summary := &Summary{}
	client := kc.client
	if client == nil {
		client = http.DefaultClient
	}
	err = kc.makeRequestAndGetValue(client, req.WithContext(ctx), summary)
	return decodeBatch(summary), err
}

func (kc *kubeletClient) getBuffer() *bytes.Buffer {
	return kc.buffers.Get().(*bytes.Buffer)
}

func (kc *kubeletClient) returnBuffer(b *bytes.Buffer) {
	b.Reset()
	kc.buffers.Put(b)
}
