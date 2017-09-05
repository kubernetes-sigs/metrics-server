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

package util

import (
	"fmt"

	"github.com/kubernetes-incubator/metrics-server/metrics/core"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/metrics/pkg/apis/metrics"
)

func ParseResourceList(ms *core.MetricSet) (metrics.ResourceList, error) {
	cpu, found := ms.MetricValues[core.MetricCpuUsageRate.MetricDescriptor.Name]
	if !found {
		return metrics.ResourceList{}, fmt.Errorf("cpu not found")
	}
	mem, found := ms.MetricValues[core.MetricMemoryWorkingSet.MetricDescriptor.Name]
	if !found {
		return metrics.ResourceList{}, fmt.Errorf("memory not found")
	}

	return metrics.ResourceList{
		metrics.ResourceName(v1.ResourceCPU.String()): *resource.NewMilliQuantity(
			cpu.IntValue,
			resource.DecimalSI),
		metrics.ResourceName(v1.ResourceMemory.String()): *resource.NewQuantity(
			mem.IntValue,
			resource.BinarySI),
	}, nil
}
