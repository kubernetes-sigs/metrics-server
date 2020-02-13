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

package provider

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	metrics "k8s.io/metrics/pkg/apis/metrics"
)

// MetricsProvider is both a PodMetricsProvider and a NodeMetricsProvider
type MetricsProvider interface {
	PodMetricsProvider
	NodeMetricsProvider
}

// TimeSpan represents the timing information for a metric, which was
// potentially calculated over some window of time (e.g. for CPU usage rate).
type TimeInfo struct {
	// NB: we consider the earliest timestamp amongst multiple containers
	// for the purposes of determining if a metric is tained by a time
	// period, like pod startup (used by things like the HPA).

	// Timestamp is the time at which the metrics were initially collected.
	// In the case of a rate metric, it should be the timestamp of the last
	// data point used in the calculation.  If it represents multiple metric
	// points, it should be the earliest such timestamp from all of the points.
	Timestamp time.Time

	// Window represents the window used to calculate rate metrics associated
	// with this timestamp.
	Window time.Duration
}

// PodMetricsProvider knows how to fetch metrics for the containers in a pod.
type PodMetricsProvider interface {
	// GetContainerMetrics gets the latest metrics for all containers in each listed pod,
	// returning both the metrics and the associated collection timestamp.
	// If a pod is missing, the container metrics should be nil for that pod.
	GetContainerMetrics(pods ...apitypes.NamespacedName) ([]TimeInfo, [][]metrics.ContainerMetrics, error)
}

// NodeMetricsProvider knows how to fetch metrics for a node.
type NodeMetricsProvider interface {
	// GetNodeMetrics gets the latest metrics for the given nodes,
	// returning both the metrics and the associated collection timestamp.
	// If a node is missing, the resourcelist should be nil for that node.
	GetNodeMetrics(nodes ...string) ([]TimeInfo, []corev1.ResourceList, error)
}
