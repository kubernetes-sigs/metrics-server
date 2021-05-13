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

package storage

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

// MetricsBatch is a single batch of pod, container, and node metrics from some source.
type MetricsBatch struct {
	Nodes []NodeMetricsPoint
	Pods  []PodMetricsPoint
}

// NodeMetricsPoint contains the metrics for some node at some point in time.
type NodeMetricsPoint struct {
	Name string
	MetricsPoint
}

// PodMetricsPoint contains the metrics for some pod's containers.
type PodMetricsPoint struct {
	Name      string
	Namespace string

	Containers []ContainerMetricsPoint
}

// ContainerMetricsPoint contains the metrics for some container at some point in time.
type ContainerMetricsPoint struct {
	Name string
	MetricsPoint
}

// MetricsPoint represents the a set of specific metrics at some point in time.
type MetricsPoint struct {
	// StartTime is the start time of container/node. Cumulative CPU usage at that moment should be equal zero.
	StartTime time.Time
	// Timestamp is the time when metric point was measured. If CPU and Memory was measured at different time it should equal CPU time to allow accurate CPU calculation.
	Timestamp time.Time
	// CumulativeCpuUsed is the cumulative cpu used at Timestamp from the StartTime of container/node. Unit: core * seconds.
	CumulativeCpuUsed resource.Quantity
	// MemoryUsage is the working set size. Unit: bytes.
	MemoryUsage resource.Quantity
}

func cpuUsageOverTime(last, prev MetricsPoint) (*resource.Quantity, error) {
	window := last.Timestamp.Sub(prev.Timestamp).Seconds()
	lastUsage := last.CumulativeCpuUsed.ScaledValue(-9)
	prevUsage := prev.CumulativeCpuUsed.ScaledValue(-9)
	if lastUsage-prevUsage < 0 {
		return nil, fmt.Errorf("Unexpected decrease in cumulative CPU usage value")
	}

	cpuUsage := float64(lastUsage-prevUsage) / window
	return resource.NewScaledQuantity(int64(cpuUsage), -9), nil
}
