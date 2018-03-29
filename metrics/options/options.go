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

package options

import (
	"time"

	"github.com/spf13/pflag"

	genericoptions "k8s.io/apiserver/pkg/server/options"
)

type HeapsterRunOptions struct {
	// genericoptions.ReccomendedOptions - EtcdOptions
	SecureServing  *genericoptions.SecureServingOptions
	Authentication *genericoptions.DelegatingAuthenticationOptions
	Authorization  *genericoptions.DelegatingAuthorizationOptions
	Features       *genericoptions.FeatureOptions

	Kubeconfig string

	// Only to be used to for testing
	DisableAuthForTesting bool

	MetricResolution    time.Duration
	Port                int
	Ip                  string
	MaxProcs            int
	KubeletPort         int
	InsecureKubelet     bool
	Version             bool
	LabelSeperator      string
	DisableMetricExport bool
}

func NewHeapsterRunOptions() *HeapsterRunOptions {
	return &HeapsterRunOptions{
		SecureServing:  genericoptions.NewSecureServingOptions(),
		Authentication: genericoptions.NewDelegatingAuthenticationOptions(),
		Authorization:  genericoptions.NewDelegatingAuthorizationOptions(),
		Features:       genericoptions.NewFeatureOptions(),
	}
}

func (h *HeapsterRunOptions) AddFlags(fs *pflag.FlagSet) {
	h.SecureServing.AddFlags(fs)
	h.Authentication.AddFlags(fs)
	h.Authorization.AddFlags(fs)
	h.Features.AddFlags(fs)

	fs.DurationVar(&h.MetricResolution, "metric_resolution", 60*time.Second, "The resolution at which heapster will retain metrics.")

	fs.IntVar(&h.MaxProcs, "max_procs", 0, "max number of CPUs that can be used simultaneously. Less than 1 for default (number of cores)")
	fs.BoolVar(&h.Version, "version", false, "print version info and exit")
	fs.StringVar(&h.LabelSeperator, "label_seperator", ",", "seperator used for joining labels")
	fs.BoolVar(&h.DisableMetricExport, "disable_export", false, "Disable exporting metrics in api/v1/metric-export")
	fs.BoolVar(&h.InsecureKubelet, "kubelet-insecure", false, "Do not connect to Kubelet using HTTPS")
	fs.IntVar(&h.KubeletPort, "kubelet-port", 10250, "The port to use to connect to Kubelets (defaults to 10250)")
	fs.StringVar(&h.Kubeconfig, "kubeconfig", "", "The path to the kubeconfig used to connect to the Kubernetes API server and the Kubelets (defaults to in-cluster config)")
}
