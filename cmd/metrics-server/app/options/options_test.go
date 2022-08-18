// Copyright 2022 The Kubernetes Authors.
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

	"k8s.io/component-base/logs"
)

func TestOptions_validate(t *testing.T) {
	for _, tc := range []struct {
		name               string
		options            *Options
		expectedErrorCount int
	}{
		{
			name: "can give --metric-resolution larger than --kubelet-request-timeout",
			options: &Options{
				MetricResolution: 10 * time.Second,
				KubeletClient:    &KubeletClientOptions{KubeletRequestTimeout: 9 * time.Second},
				Logging:          logs.NewOptions(),
			},
			expectedErrorCount: 0,
		},
		{
			name: "can not give --metric-resolution * 9/10 less than --kubelet-request-timeout",
			options: &Options{
				MetricResolution: 10 * time.Second,
				KubeletClient:    &KubeletClientOptions{KubeletRequestTimeout: 10 * time.Second},
				Logging:          logs.NewOptions(),
			},
			expectedErrorCount: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errors := tc.options.validate()
			if len(errors) != tc.expectedErrorCount {
				t.Errorf("options.Validate() = %q, expected length %d", errors, tc.expectedErrorCount)
			}
		})
	}
}
