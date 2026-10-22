// Copyright The Kubernetes Authors.
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
	"testing"
	"time"
)

func TestMetricResolutionValidationMessages(t *testing.T) {
	for _, tc := range []struct {
		name                string
		resolution, timeout time.Duration
		expected            string
	}{
		{"minimum duration", 5 * time.Second, time.Second, "metric-resolution should be a duration of at least 10s; got 5s"},
		{"request timeout", 10 * time.Second, 10 * time.Second, "metric-resolution should be larger than kubelet-request-timeout; got metric-resolution=10s and kubelet-request-timeout=10s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := &Options{MetricResolution: tc.resolution, KubeletClient: &KubeletClientOptions{KubeletRequestTimeout: tc.timeout}}
			errors := opts.validate()
			if len(errors) != 1 {
				t.Fatalf("validate() returned %v; want one error", errors)
			}
			if errors[0].Error() != tc.expected {
				t.Fatalf("got %q; want %q", errors[0].Error(), tc.expected)
			}
		})
	}
}
