// Copyright 2016 Google Inc. All Rights Reserved.
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

	"github.com/kubernetes-incubator/metrics-server/common/flags"
	genericoptions "k8s.io/apiserver/pkg/server/options"
)

type HeapsterRunOptions struct {
	// genericoptions.ReccomendedOptions - EtcdOptions
	SecureServing  *genericoptions.SecureServingOptions
	Authentication *genericoptions.DelegatingAuthenticationOptions
	Authorization  *genericoptions.DelegatingAuthorizationOptions
	Features       *genericoptions.FeatureOptions

	// Only to be used to for testing
	DisableAuthForTesting bool

	MetricResolution    time.Duration
	Port                int
	Ip                  string
	MaxProcs            int
	Sources             flags.Uris
	Sinks               flags.Uris
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

	fs.Var(&h.Sources, "source", "source(s) to watch")
	fs.Var(&h.Sinks, "sink", "external sink(s) that receive data")
	fs.DurationVar(&h.MetricResolution, "metric_resolution", 60*time.Second, "The resolution at which heapster will retain metrics.")

	fs.IntVar(&h.Port, "heapster-port", 8082, "port used by the Heapster-specific APIs")
	fs.StringVar(&h.Ip, "listen_ip", "", "IP to listen on, defaults to all IPs")
	fs.IntVar(&h.MaxProcs, "max_procs", 0, "max number of CPUs that can be used simultaneously. Less than 1 for default (number of cores)")
	fs.BoolVar(&h.Version, "version", false, "print version info and exit")
	fs.StringVar(&h.LabelSeperator, "label_seperator", ",", "seperator used for joining labels")
	fs.BoolVar(&h.DisableMetricExport, "disable_export", false, "Disable exporting metrics in api/v1/metric-export")
}
