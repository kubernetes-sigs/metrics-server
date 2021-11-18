// Copyright 2021 The Kubernetes Authors.
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

package resource

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/metrics-server/pkg/storage"
)

func TestDecode(t *testing.T) {
	tcs := []struct {
		name          string
		input         string
		defaultTime   time.Time
		expectMetrics *storage.MetricsBatch
	}{
		{
			name: "Normal",
			input: `
# HELP container_cpu_usage_seconds_total [ALPHA] Cumulative cpu time consumed by the container in core-seconds
# TYPE container_cpu_usage_seconds_total counter
container_cpu_usage_seconds_total{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 4.710169 1633253812125
# HELP container_memory_working_set_bytes [ALPHA] Current working set of the container in bytes
# TYPE container_memory_working_set_bytes gauge
container_memory_working_set_bytes{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 1.253376e+07 1633253812125
# TYPE container_start_time_seconds gauge
container_start_time_seconds{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 1.633252712e+9 1633253812125
# HELP node_cpu_usage_seconds_total [ALPHA] Cumulative cpu time consumed by the node in core-seconds
# TYPE node_cpu_usage_seconds_total counter
node_cpu_usage_seconds_total 357.35491 1633253809720
# HELP node_memory_working_set_bytes [ALPHA] Current working set of the node in bytes
# TYPE node_memory_working_set_bytes gauge
node_memory_working_set_bytes 1.616273408e+09 1633253809720
# HELP pod_cpu_usage_seconds_total [ALPHA] Cumulative cpu time consumed by the pod in core-seconds
# TYPE pod_cpu_usage_seconds_total counter
pod_cpu_usage_seconds_total{namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 4.67812 1633253803935
# HELP pod_memory_working_set_bytes [ALPHA] Current working set of the pod in bytes
# TYPE pod_memory_working_set_bytes gauge
pod_memory_working_set_bytes{namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 1.2627968e+07 1633253803935
# HELP scrape_error [ALPHA] 1 if there was an error while getting container metrics, 0 otherwise
# TYPE scrape_error gauge
scrape_error 0
`,
			expectMetrics: &storage.MetricsBatch{
				Nodes: map[string]storage.MetricsPoint{
					"node1": {
						Timestamp:         time.Date(2021, 10, 3, 9, 36, 49, 720000000, time.UTC),
						CumulativeCpuUsed: 357354910000,
						MemoryUsage:       1616273408,
					},
				},
				Pods: map[apitypes.NamespacedName]storage.PodMetricsPoint{
					{Name: "coredns-558bd4d5db-4dpjz", Namespace: "kube-system"}: {
						Containers: map[string]storage.MetricsPoint{
							"coredns": {
								Timestamp:         time.Date(2021, 10, 3, 9, 36, 52, 125000000, time.UTC),
								CumulativeCpuUsed: 4710169000,
								MemoryUsage:       12533760,
								StartTime:         time.Date(2021, 10, 3, 9, 18, 32, 0, time.UTC),
							},
						},
					},
				},
			},
		},
		{
			name: "Without timestamp uses defaultTime",
			input: `
container_cpu_usage_seconds_total{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 4.710169
container_memory_working_set_bytes{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 1.253376e+07
container_start_time_seconds{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 1.633252712e+9
node_cpu_usage_seconds_total 357.35491
node_memory_working_set_bytes 1.616273408e+09
`,
			defaultTime: time.Date(2077, 7, 7, 7, 7, 7, 0, time.UTC),
			expectMetrics: &storage.MetricsBatch{
				Nodes: map[string]storage.MetricsPoint{
					"node1": {
						Timestamp:         time.Date(2077, 7, 7, 7, 7, 7, 0, time.UTC),
						CumulativeCpuUsed: 357354910000,
						MemoryUsage:       1616273408,
					},
				},
				Pods: map[apitypes.NamespacedName]storage.PodMetricsPoint{
					{Name: "coredns-558bd4d5db-4dpjz", Namespace: "kube-system"}: {
						Containers: map[string]storage.MetricsPoint{
							"coredns": {
								Timestamp:         time.Date(2077, 7, 7, 7, 7, 7, 0, time.UTC),
								CumulativeCpuUsed: 4710169000,
								MemoryUsage:       12533760,
								StartTime:         time.Date(2021, 10, 3, 9, 18, 32, 0, time.UTC),
							},
						},
					},
				},
			},
		},
		{
			name: "Single node",
			input: `
container_cpu_usage_seconds_total{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 4.710169 1633253812125
container_memory_working_set_bytes{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 1.253376e+07 1633253812125
container_start_time_seconds{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 1.633252712e+9 1633253812125
`,
			expectMetrics: &storage.MetricsBatch{
				Nodes: map[string]storage.MetricsPoint{},
				Pods: map[apitypes.NamespacedName]storage.PodMetricsPoint{
					{Name: "coredns-558bd4d5db-4dpjz", Namespace: "kube-system"}: {
						Containers: map[string]storage.MetricsPoint{
							"coredns": {
								Timestamp:         time.Date(2021, 10, 3, 9, 36, 52, 125000000, time.UTC),
								CumulativeCpuUsed: 4710169000,
								MemoryUsage:       12533760,
								StartTime:         time.Date(2021, 10, 3, 9, 18, 32, 0, time.UTC),
							},
						},
					},
				},
			},
		},
		{
			name: "No container CPU drops container metrics",
			input: `
container_memory_working_set_bytes{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 1.253376e+07 1633253812125
`,
			expectMetrics: nil,
		},
		{
			name: "Empty container CPU drops container metrics",
			input: `
container_cpu_usage_seconds_total{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 0 1633253812125
container_memory_working_set_bytes{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 1.253376e+07 1633253812125
`,
			expectMetrics: nil,
		},
		{
			name: "No container Memory drops container metrics",
			input: `
container_cpu_usage_seconds_total{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 4.710169 1633253812125
`,
			expectMetrics: nil,
		},
		{
			name: "Empty container Memory drops container metrics",
			input: `
container_cpu_usage_seconds_total{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 4.710169 1633253812125
container_memory_working_set_bytes{container="coredns",namespace="kube-system",pod="coredns-558bd4d5db-4dpjz"} 0 1633253812125
`,
			expectMetrics: nil,
		},
		{
			name: "Single node",
			input: `
node_cpu_usage_seconds_total 357.35491 1633253809720
node_memory_working_set_bytes 1.616273408e+09 1633253809720
`,
			expectMetrics: &storage.MetricsBatch{
				Nodes: map[string]storage.MetricsPoint{
					"node1": {
						Timestamp:         time.Date(2021, 10, 3, 9, 36, 49, 720000000, time.UTC),
						CumulativeCpuUsed: 357354910000,
						MemoryUsage:       1616273408,
					},
				},
				Pods: map[apitypes.NamespacedName]storage.PodMetricsPoint{},
			},
		},
		{
			name: "No node CPU drops metric",
			input: `
node_memory_working_set_bytes 1.616273408e+09 1633253809720
`,
			expectMetrics: nil,
		},
		{
			name: "Empty node CPU drops metric",
			input: `
node_cpu_usage_seconds_total 0 1633253809720
node_memory_working_set_bytes 1.616273408e+09 1633253809720
`,
			expectMetrics: nil,
		},
		{
			name: "No node Memory drops metrics",
			input: `
node_cpu_usage_seconds_total 357.35491 1633253809720
`,
			expectMetrics: nil,
		},
		{
			name: "Empty node Memory drops metric",
			input: `
node_cpu_usage_seconds_total 357.35491 1633253809720
node_memory_working_set_bytes 0 1633253809720
`,
			expectMetrics: nil,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectMetrics == nil {
				tc.expectMetrics = &storage.MetricsBatch{
					Nodes: map[string]storage.MetricsPoint{},
					Pods:  map[apitypes.NamespacedName]storage.PodMetricsPoint{},
				}
			}
			ms := decodeBatch([]byte(tc.input), tc.defaultTime, "node1")
			if diff := cmp.Diff(tc.expectMetrics, ms); diff != "" {
				t.Errorf(`Metrics diff: %s`, diff)
			}
		})
	}
}
