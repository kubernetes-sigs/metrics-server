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

package sources

import (
	"context"
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
	Timestamp time.Time
	// CpuUsage is the CPU usage rate, in cores
	CpuUsage resource.Quantity
	// MemoryUsage is the working set size, in bytes.
	MemoryUsage resource.Quantity
}

// MetricSource knows how to collect pod, container, and node metrics from some location.
// It is expected that the batch returned contains unique values (i.e. it does not return
// the same node, pod, or container as any other source).
type MetricSource interface {
	// Collect fetches a batch of metrics.  It may return both a partial result and an error,
	// and non-nil results thus must be well-formed and meaningful even when accompanied by 
	// and error.
	Collect(context.Context) (*MetricsBatch, error)
	// Name names the metrics source for identification purposes
	Name() string
}

// MetricSourceProvider provides metric sources to collect from.
type MetricSourceProvider interface {
	// GetMetricSources fetches all sources known to this metrics provider.
	// It may return both partial results and an error.
	GetMetricSources() ([]MetricSource, error)
}
