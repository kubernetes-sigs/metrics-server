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
	"reflect"
	"testing"
	"time"

	"sigs.k8s.io/metrics-server/pkg/api"

	v1 "k8s.io/api/core/v1"
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

func Test_resourceUsage(t *testing.T) {
	start := time.Now()
	tcs := []struct {
		name             string
		last             MetricsPoint
		prev             MetricsPoint
		wantResourceList v1.ResourceList
		wantTimeInfo     api.TimeInfo
		wantErr          bool
	}{
		{
			name: "get resource usage successfully",
			last: newMetricsPoint(start, start.Add(20*time.Millisecond), 50000000, 600),
			prev: newMetricsPoint(start, start.Add(10*time.Millisecond), 30000000, 400),
			wantResourceList: v1.ResourceList{v1.ResourceCPU: uint64Quantity(uint64(2000), resource.DecimalSI, -3),
				v1.ResourceMemory: uint64Quantity(600, resource.BinarySI, 0)},
			wantTimeInfo: api.TimeInfo{Timestamp: start.Add(20 * time.Millisecond), Window: 10 * time.Millisecond},
		},
		{
			name:             "get resource usage failed because of unexpected decrease in startTime",
			last:             newMetricsPoint(start, start.Add(20*time.Millisecond), 500, 600),
			prev:             newMetricsPoint(start.Add(20*time.Millisecond), start.Add(10*time.Millisecond), 300, 400),
			wantResourceList: v1.ResourceList{},
			wantTimeInfo:     api.TimeInfo{},
			wantErr:          true,
		},
		{
			name:             "get resource usage failed because of unexpected decrease in cumulative CPU usage value",
			last:             newMetricsPoint(start, start.Add(20*time.Millisecond), 100, 600),
			prev:             newMetricsPoint(start, start.Add(10*time.Millisecond), 300, 400),
			wantResourceList: v1.ResourceList{},
			wantTimeInfo:     api.TimeInfo{},
			wantErr:          true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			resourceList, timeInfo, err := resourceUsage(tc.last, tc.prev)
			if (err != nil) != tc.wantErr {
				t.Errorf("resourceUsage() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if !reflect.DeepEqual(resourceList, tc.wantResourceList) {
				t.Errorf("resourceUsage() resourceList = %v, want %v", resourceList, tc.wantResourceList)
			}
			if !reflect.DeepEqual(timeInfo, tc.wantTimeInfo) {
				t.Errorf("resourceUsage() timeInfo = %v, want %v", timeInfo, tc.wantTimeInfo)
			}
		})
	}
}
