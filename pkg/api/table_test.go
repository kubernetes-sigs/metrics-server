/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/metrics/pkg/apis/metrics"
)

func TestNodeList_ConvertToTable(t *testing.T) {
	// setup
	r := NewTestNodeStorage(nil)

	// execute
	got, err := r.List(genericapirequest.NewContext(), nil)

	// assert
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	res, err := r.ConvertToTable(genericapirequest.NewContext(), got, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(res.Rows) != 3 ||
		res.ColumnDefinitions[1].Name != "res1" || res.ColumnDefinitions[2].Name != "Window" ||
		res.Rows[0].Cells[0] != "node1" ||
		res.Rows[0].Cells[1] != "10m" ||
		res.Rows[0].Cells[2] != "1µs" ||
		res.Rows[1].Cells[0] != "node2" ||
		res.Rows[1].Cells[1] != "5Mi" ||
		res.Rows[1].Cells[2] != "2µs" ||
		res.Rows[2].Cells[0] != "node3" ||
		res.Rows[2].Cells[1] != "1" ||
		res.Rows[2].Cells[2] != "3µs" {
		t.Errorf("Got unexpected object: %+v", res)
	}
}

func TestPodList_ConvertToTable(t *testing.T) {
	// setup
	r := NewPodTestStorage(nil)

	// execute
	got, err := r.List(genericapirequest.NewContext(), nil)

	// assert
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	res, err := r.ConvertToTable(genericapirequest.NewContext(), got, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(res.Rows) != 3 ||
		res.ColumnDefinitions[1].Name != "cpu" || res.ColumnDefinitions[2].Name != "memory" || res.ColumnDefinitions[3].Name != "Window" ||
		res.Rows[0].Cells[0] != "pod1" ||
		res.Rows[0].Cells[1] != "10m" ||
		res.Rows[0].Cells[2] != "5Mi" ||
		res.Rows[0].Cells[3] != "1µs" ||
		res.Rows[1].Cells[0] != "pod2" ||
		res.Rows[1].Cells[1] != "20m" ||
		res.Rows[1].Cells[2] != "15Mi" ||
		res.Rows[1].Cells[3] != "2µs" ||
		res.Rows[2].Cells[0] != "pod3" ||
		res.Rows[2].Cells[1] != "20m" ||
		res.Rows[2].Cells[2] != "25Mi" ||
		res.Rows[2].Cells[3] != "3µs" {
		t.Errorf("Got unexpected object: %+v", res)
	}
}

func TestAddMetricsToTableResourceColumns(t *testing.T) {
	tcs := []struct {
		name        string
		add         func(table *metav1beta1.Table)
		wantColumns []string
		wantCells   [][]any
	}{
		{
			name: "pod resources missing from the first row still get a column",
			add: func(table *metav1beta1.Table) {
				addPodMetricsToTable(table,
					tablePod("pod-a", time.Microsecond, corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10m")}),
					tablePod("pod-b", 2*time.Microsecond, corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("5Mi")}))
			},
			wantColumns: []string{"Name", "cpu", "memory", "Window"},
			wantCells: [][]any{
				{"pod-a", "10m", "0", "1µs"},
				{"pod-b", "0", "5Mi", "2µs"},
			},
		},
		{
			// podStorage.GetMetrics returns a PodMetrics without containers when
			// every container was skipped, so this is a normal List result.
			name: "pod without container metrics keeps the row arity",
			add: func(table *metav1beta1.Table) {
				addPodMetricsToTable(table,
					tablePod("pod-a", 0, nil),
					tablePod("pod-b", 2*time.Microsecond, corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("20m"), corev1.ResourceMemory: resource.MustParse("15Mi")}))
			},
			wantColumns: []string{"Name", "cpu", "memory", "Window"},
			wantCells: [][]any{
				{"pod-a", "0", "0", "0s"},
				{"pod-b", "20m", "15Mi", "2µs"},
			},
		},
		{
			name: "node resources missing from the first row still get a column",
			add: func(table *metav1beta1.Table) {
				addNodeMetricsToTable(table,
					tableNode("node-a", time.Microsecond, corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10m")}),
					tableNode("node-b", 2*time.Microsecond, corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("5Mi")}))
			},
			wantColumns: []string{"Name", "cpu", "memory", "Window"},
			wantCells: [][]any{
				{"node-a", "10m", "0", "1µs"},
				{"node-b", "0", "5Mi", "2µs"},
			},
		},
		{
			name: "node without usage keeps the row arity",
			add: func(table *metav1beta1.Table) {
				addNodeMetricsToTable(table,
					tableNode("node-a", 0, nil),
					tableNode("node-b", 2*time.Microsecond, corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("20m"), corev1.ResourceMemory: resource.MustParse("15Mi")}))
			},
			wantColumns: []string{"Name", "cpu", "memory", "Window"},
			wantCells: [][]any{
				{"node-a", "0", "0", "0s"},
				{"node-b", "20m", "15Mi", "2µs"},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			var table metav1beta1.Table
			tc.add(&table)

			columns := make([]string, 0, len(table.ColumnDefinitions))
			for _, column := range table.ColumnDefinitions {
				columns = append(columns, column.Name)
			}
			if diff := cmp.Diff(tc.wantColumns, columns); diff != "" {
				t.Errorf("unexpected columns, diff: %s", diff)
			}
			if len(table.Rows) != len(tc.wantCells) {
				t.Fatalf("got %d rows, want %d", len(table.Rows), len(tc.wantCells))
			}
			for i, row := range table.Rows {
				if len(row.Cells) != len(table.ColumnDefinitions) {
					t.Errorf("row %d has %d cells but the table defines %d columns", i, len(row.Cells), len(table.ColumnDefinitions))
				}
				if diff := cmp.Diff(tc.wantCells[i], row.Cells); diff != "" {
					t.Errorf("unexpected cells for row %d, diff: %s", i, diff)
				}
			}
		})
	}
}

func tablePod(name string, window time.Duration, usage corev1.ResourceList) metrics.PodMetrics {
	pod := metrics.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Window:     metav1.Duration{Duration: window},
	}
	if usage != nil {
		pod.Containers = []metrics.ContainerMetrics{{Name: "c1", Usage: usage}}
	}
	return pod
}

func tableNode(name string, window time.Duration, usage corev1.ResourceList) metrics.NodeMetrics {
	return metrics.NodeMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Window:     metav1.Duration{Duration: window},
		Usage:      usage,
	}
}
