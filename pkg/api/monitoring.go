// Copyright 2020 The Kubernetes Authors.
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

package api

import (
	"k8s.io/component-base/metrics"
)

var (
	metricFreshness = metrics.NewHistogramVec(
		&metrics.HistogramOpts{
			Namespace: "metrics_server",
			Subsystem: "api",
			Name:      "metric_freshness_seconds",
			Help:      "Freshness of metrics exported",
			Buckets:   metrics.ExponentialBuckets(1, 1.364, 20),
		},
		[]string{},
	)
)

// RegisterAPIMetrics registers a histogram metric for the freshness of
// exported metrics.
func RegisterAPIMetrics(registrationFunc func(metrics.Registerable) error) error {
	return registrationFunc(metricFreshness)
}
