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
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"k8s.io/klog"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

// KubeletInterface knows how to fetch metrics from the Kubelet
type KubeletInterface interface {
	// GetSummary fetches summary metrics from the given Kubelet
	GetSummary(ctx context.Context, host string) (*stats.Summary, error)
}

type kubeletClient struct {
	port            int
	deprecatedNoTLS bool
	client          *http.Client
}

type ErrNotFound struct {
	endpoint string
}

func (err *ErrNotFound) Error() string {
	return fmt.Sprintf("%q not found", err.endpoint)
}

func IsNotFoundError(err error) bool {
	_, isNotFound := err.(*ErrNotFound)
	return isNotFound
}

func (kc *kubeletClient) makeRequestAndGetValue(client *http.Client, req *http.Request, value interface{}) error {
	// TODO(directxman12): support validating certs by hostname
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body - %v", err)
	}
	if response.StatusCode == http.StatusNotFound {
		return &ErrNotFound{req.URL.String()}
	} else if response.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed - %q, response: %q", response.Status, string(body))
	}

	kubeletAddr := "[unknown]"
	if req.URL != nil {
		kubeletAddr = req.URL.Host
	}
	klog.V(10).Infof("Raw response from Kubelet at %s: %s", kubeletAddr, string(body))

	err = json.Unmarshal(body, value)
	if err != nil {
		return fmt.Errorf("failed to parse output. Response: %q. Error: %v", string(body), err)
	}
	return nil
}

func (kc *kubeletClient) GetSummary(ctx context.Context, host string) (*stats.Summary, error) {
	scheme := "https"
	if kc.deprecatedNoTLS {
		scheme = "http"
	}
	url := url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(host, strconv.Itoa(kc.port)),
		Path:   "/stats/summary/",
	}

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, err
	}
	summary := &stats.Summary{}
	client := kc.client
	if client == nil {
		client = http.DefaultClient
	}
	err = kc.makeRequestAndGetValue(client, req.WithContext(ctx), summary)
	return summary, err
}

func NewKubeletClient(transport http.RoundTripper, port int, deprecatedNoTLS bool) (KubeletInterface, error) {
	c := &http.Client{
		Transport: transport,
	}
	return &kubeletClient{
		port:            port,
		client:          c,
		deprecatedNoTLS: deprecatedNoTLS,
	}, nil
}
