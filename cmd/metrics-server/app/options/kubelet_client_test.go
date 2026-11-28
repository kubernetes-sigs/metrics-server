// Copyright 2020 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package options

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/metrics-server/pkg/scraper/client"
)

func TestConfig(t *testing.T) {
	kubeconfig := &rest.Config{
		Host:            "https://10.96.0.1:443",
		APIPath:         "",
		Username:        "Username",
		Password:        "Password",
		BearerToken:     "ApiserverBearerToken",
		BearerTokenFile: "ApiserverBearerTokenFile",
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: false,
			CertFile: "CertFile",
			KeyFile:  "KeyFile",
			CAFile:   "CAFile",
			CertData: []byte("CertData"),
			KeyData:  []byte("KeyData"),
			CAData:   []byte("CAData"),
		},
		UserAgent: "UserAgent",
	}

	expected := client.KubeletClientConfig{
		AddressTypePriority: []v1.NodeAddressType{"Hostname", "InternalDNS", "InternalIP", "ExternalDNS", "ExternalIP"},
		Scheme:              "https",
		DefaultPort:         10250,
		Client:              *kubeconfig,
	}

	for _, tc := range []struct {
		name        string
		optionsFunc func() *KubeletClientOptions
		expectFunc  func() client.KubeletClientConfig
		kubeconfig  *rest.Config
	}{
		{
			name:        "Default configuration should use config from kubeconfig",
			optionsFunc: NewKubeletClientOptions,
			expectFunc: func() client.KubeletClientConfig {
				return expected
			},
		},
		{
			name: "InsecureKubeletTLS removes CA config and sets insecure",
			optionsFunc: func() *KubeletClientOptions {
				o := NewKubeletClientOptions()
				o.InsecureKubeletTLS = true
				return o
			},
			expectFunc: func() client.KubeletClientConfig {
				e := expected
				e.Client.Insecure = true
				e.Client.CAFile = ""
				e.Client.CAData = nil
				return e
			},
		},
		{
			name: "KubeletCAFile overrides CA file and data",
			optionsFunc: func() *KubeletClientOptions {
				o := NewKubeletClientOptions()
				o.KubeletCAFile = "Override"
				return o
			},
			expectFunc: func() client.KubeletClientConfig {
				e := expected
				e.Client.CAFile = "Override"
				e.Client.CAData = nil
				return e
			},
		},
		{
			name: "DeprecatedCompletelyInsecureKubelet resets TLSConfig and sets https scheme",
			optionsFunc: func() *KubeletClientOptions {
				o := NewKubeletClientOptions()
				o.DeprecatedCompletelyInsecureKubelet = true
				return o
			},
			expectFunc: func() client.KubeletClientConfig {
				e := expected
				e.Client.TLSClientConfig = rest.TLSClientConfig{}
				e.Client.Username = ""
				e.Client.Password = ""
				e.Client.BearerToken = ""
				e.Client.BearerTokenFile = ""
				e.Scheme = "http"
				return e
			},
		},
		{
			name: "KubeletClientCertFile overrides TLS client cert file",
			optionsFunc: func() *KubeletClientOptions {
				o := NewKubeletClientOptions()
				o.KubeletClientCertFile = "Override"
				return o
			},
			expectFunc: func() client.KubeletClientConfig {
				e := expected
				e.Client.TLSClientConfig.CertFile = "Override"
				e.Client.TLSClientConfig.CertData = nil
				return e
			},
		},
		{
			name: "KubeletClientKeyFile overrides TLS client key file",
			optionsFunc: func() *KubeletClientOptions {
				o := NewKubeletClientOptions()
				o.KubeletClientKeyFile = "Override"
				return o
			},
			expectFunc: func() client.KubeletClientConfig {
				e := expected
				e.Client.TLSClientConfig.KeyFile = "Override"
				e.Client.TLSClientConfig.KeyData = nil
				return e
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := tc.optionsFunc().Config(kubeconfig)
			if diff := cmp.Diff(*config, tc.expectFunc()); diff != "" {
				t.Errorf("Unexpected options.KubeletConfig(), diff:\n%s", diff)
			}
		})
	}
}

var defaultAddressTypes = []string{"Hostname", "InternalDNS", "InternalIP", "ExternalDNS", "ExternalIP"}

func TestValidate(t *testing.T) {
	for _, tc := range []struct {
		name               string
		options            *KubeletClientOptions
		expectedErrorCount int
	}{
		{
			name:               "Default options should pass validate",
			options:            NewKubeletClientOptions(),
			expectedErrorCount: 0,
		},
		{
			name: "Cannot use both --kubelet-certificate-authority and --deprecated-kubelet-completely-insecure",
			options: &KubeletClientOptions{
				KubeletPort:                         10250,
				KubeletPreferredAddressTypes:        defaultAddressTypes,
				DeprecatedCompletelyInsecureKubelet: true,
				KubeletCAFile:                       "a",
				KubeletRequestTimeout:               1 * time.Second,
			},
			expectedErrorCount: 1,
		},
		{
			name: "Cannot use both --kubelet-certificate-authority and --kubelet-insecure-tls",
			options: &KubeletClientOptions{
				KubeletPort:                  10250,
				KubeletPreferredAddressTypes: defaultAddressTypes,
				InsecureKubeletTLS:           true,
				KubeletCAFile:                "a",
				KubeletRequestTimeout:        1 * time.Second,
			},
			expectedErrorCount: 1,
		},
		{
			name: "use both --kubelet-client-certificate and --kubelet-client-key",
			options: &KubeletClientOptions{
				KubeletPort:                  10250,
				KubeletPreferredAddressTypes: defaultAddressTypes,
				KubeletClientKeyFile:         "a",
				KubeletClientCertFile:        "b",
				KubeletRequestTimeout:        1 * time.Second,
			},
			expectedErrorCount: 0,
		},
		{
			name: "cannot use both --kubelet-client-certificate and --deprecated-kubelet-completely-insecure",
			options: &KubeletClientOptions{
				KubeletPort:                         10250,
				KubeletPreferredAddressTypes:        defaultAddressTypes,
				KubeletClientCertFile:               "a",
				DeprecatedCompletelyInsecureKubelet: true,
				KubeletRequestTimeout:               1 * time.Second,
			},
			expectedErrorCount: 2,
		},
		{
			name: "cannot use both --kubelet-client-key and --deprecated-kubelet-completely-insecure",
			options: &KubeletClientOptions{
				KubeletPort:                         10250,
				KubeletPreferredAddressTypes:        defaultAddressTypes,
				KubeletClientKeyFile:                "a",
				DeprecatedCompletelyInsecureKubelet: true,
				KubeletRequestTimeout:               1 * time.Second,
			},
			expectedErrorCount: 2,
		},
		{
			name: "cannot use both --kubelet-insecure-tls and --deprecated-kubelet-completely-insecure",
			options: &KubeletClientOptions{
				KubeletPort:                         10250,
				KubeletPreferredAddressTypes:        defaultAddressTypes,
				InsecureKubeletTLS:                  true,
				DeprecatedCompletelyInsecureKubelet: true,
				KubeletRequestTimeout:               1 * time.Second,
			},
			expectedErrorCount: 1,
		},
		{
			name: "cannot give only --kubelet-client-key, give --kubelet-key-file as well",
			options: &KubeletClientOptions{
				KubeletPort:                  10250,
				KubeletPreferredAddressTypes: defaultAddressTypes,
				KubeletClientCertFile:        "a",
				KubeletRequestTimeout:        1 * time.Second,
			},
			expectedErrorCount: 1,
		},
		{
			name: "cannot give only --kubelet-client-key, give --kubelet-certificate-authority as well",
			options: &KubeletClientOptions{
				KubeletPort:                  10250,
				KubeletPreferredAddressTypes: defaultAddressTypes,
				KubeletClientKeyFile:         "a",
				KubeletRequestTimeout:        1 * time.Second,
			},
			expectedErrorCount: 1,
		},
		{
			name: "can give --kubelet-request-timeout value larger than 0",
			options: &KubeletClientOptions{
				KubeletPort:                  10250,
				KubeletPreferredAddressTypes: defaultAddressTypes,
				KubeletRequestTimeout:        1 * time.Second,
			},
			expectedErrorCount: 0,
		},
		{
			name: "cannot give --kubelet-request-timeout value equal 0",
			options: &KubeletClientOptions{
				KubeletPort:                  10250,
				KubeletPreferredAddressTypes: defaultAddressTypes,
				KubeletRequestTimeout:        0 * time.Second,
			},
			expectedErrorCount: 1,
		},
		{
			name: "cannot give --kubelet-request-timeout value less than 0",
			options: &KubeletClientOptions{
				KubeletPort:                  10250,
				KubeletPreferredAddressTypes: defaultAddressTypes,
				KubeletRequestTimeout:        -10 * time.Second,
			},
			expectedErrorCount: 1,
		},
		{
			name: "cannot give --kubelet-port value above 65535",
			options: func() *KubeletClientOptions {
				o := NewKubeletClientOptions()
				o.KubeletPort = 70000
				return o
			}(),
			expectedErrorCount: 1,
		},
		{
			name: "cannot give negative --kubelet-port value",
			options: func() *KubeletClientOptions {
				o := NewKubeletClientOptions()
				o.KubeletPort = -1
				return o
			}(),
			expectedErrorCount: 1,
		},
		{
			name: "cannot give --kubelet-port value 0 without --kubelet-use-node-status-port",
			options: func() *KubeletClientOptions {
				o := NewKubeletClientOptions()
				o.KubeletPort = 0
				return o
			}(),
			expectedErrorCount: 1,
		},
		{
			name: "can give --kubelet-port value 0 with --kubelet-use-node-status-port",
			options: func() *KubeletClientOptions {
				o := NewKubeletClientOptions()
				o.KubeletPort = 0
				o.KubeletUseNodeStatusPort = true
				return o
			}(),
			expectedErrorCount: 0,
		},
		{
			name: "cannot give an empty --kubelet-preferred-address-types",
			options: func() *KubeletClientOptions {
				o := NewKubeletClientOptions()
				o.KubeletPreferredAddressTypes = nil
				return o
			}(),
			expectedErrorCount: 1,
		},
		{
			name: "cannot give a blank entry in --kubelet-preferred-address-types",
			options: func() *KubeletClientOptions {
				o := NewKubeletClientOptions()
				o.KubeletPreferredAddressTypes = []string{"InternalIP", ""}
				return o
			}(),
			expectedErrorCount: 1,
		},
		{
			name: "cannot give a misspelled entry in --kubelet-preferred-address-types",
			options: func() *KubeletClientOptions {
				o := NewKubeletClientOptions()
				o.KubeletPreferredAddressTypes = []string{"InternalIp", "ExternalIP"}
				return o
			}(),
			expectedErrorCount: 1,
		},
		{
			name: "reports every unknown entry in --kubelet-preferred-address-types",
			options: func() *KubeletClientOptions {
				o := NewKubeletClientOptions()
				o.KubeletPreferredAddressTypes = []string{"internalip", "InternalIP", "externalip"}
				return o
			}(),
			expectedErrorCount: 2,
		},
		{
			name: "can give a subset of the address types in any order",
			options: func() *KubeletClientOptions {
				o := NewKubeletClientOptions()
				o.KubeletPreferredAddressTypes = []string{"ExternalDNS", "InternalIP"}
				return o
			}(),
			expectedErrorCount: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errors := tc.options.Validate()
			if len(errors) != tc.expectedErrorCount {
				t.Errorf("options.Validate() = %q, expected length %d", errors, tc.expectedErrorCount)
			}
		})
	}
}
