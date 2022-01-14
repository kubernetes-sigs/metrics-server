// Copyright 2021 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"math"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
)

func TestUint64Quantity(t *testing.T) {
	tcs := []struct {
		name   string
		input  uint64
		expect resource.Quantity
	}{
		{"math.MaxInt64 + 10", uint64(math.MaxInt64 + 10), *resource.NewScaledQuantity(int64(math.MaxInt64/10+1), 1)},
		{"math.MaxInt64 + 20", uint64(math.MaxInt64 + 20), *resource.NewScaledQuantity(int64(math.MaxInt64/10+2), 1)},
		{"math.MaxInt64 - 10", uint64(math.MaxInt64 - 10), *resource.NewScaledQuantity(int64(math.MaxInt64-10), 0)},
		{"math.MaxInt64 - 100", uint64(math.MaxInt64 - 100), *resource.NewScaledQuantity(int64(math.MaxInt64-100), 0)},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got := uint64Quantity(tc.input, resource.DecimalSI, 0)
			if got != tc.expect {
				t.Errorf("uint64Quantity(%d, resource.DecimalSI, 0) = %+v, expected: %+v", tc.input, got, tc.expect)
			}
		})
	}
}
