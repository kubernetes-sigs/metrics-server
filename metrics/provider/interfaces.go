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

	apitypes "k8s.io/apimachinery/pkg/types"
	metrics "k8s.io/metrics/pkg/apis/metrics"
)

// MetricsProvider is both a PodMetricsProvider and a NodeMetricsProvider
type MetricsProvider interface {
	PodMetricsProvider
	NodeMetricsProvider
}

// PodMetricsProvider knows how to fetch metrics for the containers in a pod.
type PodMetricsProvider interface {
	// GetContainerMetrics gets the latest metrics for all containers in a pod,
	// returning both the metrics and the associated collection timestamp.
	// It will return an errors.NotFound if the metrics aren't found.
	GetContainerMetrics(pod apitypes.NamespacedName) (time.Time, []metrics.ContainerMetrics, error)
}

// NodeMetricsProvider knows how to fetch metrics for a node.
type NodeMetricsProvider interface {
	// GetNodeMetrics gets the latest metrics for the given node,
	// returning both the metrics and the associated collection timestamp.
	GetNodeMetrics(node string) (time.Time, metrics.ResourceList, error)
}
