// Copyright 2026 The Kubernetes Authors.
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

package app

import (
	"strings"
	"testing"

	"sigs.k8s.io/metrics-server/cmd/metrics-server/app/options"
)

func TestValidateOptions(t *testing.T) {
	t.Run("default options are valid", func(t *testing.T) {
		if err := validateOptions(options.NewOptions()); err != nil {
			t.Fatalf("validateOptions() = %v, expected no error", err)
		}
	})

	t.Run("reports every invalid option", func(t *testing.T) {
		o := options.NewOptions()
		o.MetricResolution = 5e9
		o.KubeletClient.KubeletPort = 70000
		o.KubeletClient.InsecureKubeletTLS = true
		o.KubeletClient.KubeletCAFile = "ca.pem"

		err := validateOptions(o)
		if err == nil {
			t.Fatal("validateOptions() = nil, expected an error")
		}
		for _, want := range []string{"metric-resolution", "kubelet-port", "kubelet-insecure-tls"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("validateOptions() = %q, expected it to mention %q", err, want)
			}
		}
	})
}
