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

	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"

	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/metrics-server/pkg/scraper/client"
	"sigs.k8s.io/metrics-server/pkg/storage"
	"sigs.k8s.io/metrics-server/pkg/utils"

	corev1 "k8s.io/api/core/v1"
)

type kubeletClient struct {
	defaultPort       int
	useNodeStatusPort bool
	client            *http.Client
	scheme            string
	addrResolver      utils.NodeAddressResolver
	buffers           sync.Pool
}

func NewClient(config client.KubeletClientConfig) (*kubeletClient, error) {
	transport, err := rest.TransportFor(&config.Client)
	if err != nil {
		return nil, fmt.Errorf("unable to construct transport: %v", err)
	}

	c := &http.Client{
		Transport: transport,
		Timeout:   config.Client.Timeout,
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

var _ client.KubeletMetricsInterface = (*kubeletClient)(nil)

// GetMetrics get metrics from kubelet /metrics/resource endpoint
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
		Scheme: kc.scheme,
		Host:   net.JoinHostPort(addr, strconv.Itoa(port)),
		Path:   "/metrics/resource",
	}

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, err
	}
	samples, err := kc.sendRequestDecode(kc.client, req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return decodeBatch(samples, node.Name), err
}

func (kc *kubeletClient) sendRequestDecode(client *http.Client, req *http.Request) ([]*model.Sample, error) {
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed, status: %q", response.Status)
	}
	b := kc.getBuffer()
	defer kc.returnBuffer(b)
	_, err = io.Copy(b, response.Body)
	if err != nil {
		return nil, err
	}
	dec := expfmt.NewDecoder(b, expfmt.FmtText)
	decoder := expfmt.SampleDecoder{
		Dec:  dec,
		Opts: &expfmt.DecodeOptions{},
	}

	var samples []*model.Sample
	for {
		var v model.Vector
		if err := decoder.Decode(&v); err != nil {
			if err == io.EOF {
				// Expected loop termination condition.
				break
			}
			return nil, err
		}
		samples = append(samples, v...)
	}
	return samples, nil
}

func (kc *kubeletClient) getBuffer() *bytes.Buffer {
	return kc.buffers.Get().(*bytes.Buffer)
}

func (kc *kubeletClient) returnBuffer(b *bytes.Buffer) {
	b.Reset()
	kc.buffers.Put(b)
}
