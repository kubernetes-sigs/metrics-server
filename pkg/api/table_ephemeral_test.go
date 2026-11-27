// Copyright 2026 The Kubernetes Authors.
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

package api

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/metrics/pkg/apis/metrics"
)

func ephPod(name string, usage v1.ResourceList) metrics.PodMetrics {
	return metrics.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Containers: []metrics.ContainerMetrics{{Name: "c", Usage: usage}},
	}
}

func TestAddPodMetricsToTableEphemeralStorage(t *testing.T) {
	without := ephPod("a", v1.ResourceList{
		v1.ResourceCPU:    resource.MustParse("100m"),
		v1.ResourceMemory: resource.MustParse("10Mi"),
	})
	with := ephPod("b", v1.ResourceList{
		v1.ResourceCPU:              resource.MustParse("200m"),
		v1.ResourceMemory:           resource.MustParse("20Mi"),
		v1.ResourceEphemeralStorage: resource.MustParse("4Ki"),
	})

	var table metav1beta1.Table
	addPodMetricsToTable(&table, without, with) // first pod has no ephemeral-storage

	var cols []string
	for _, c := range table.ColumnDefinitions {
		cols = append(cols, c.Name)
	}
	wantCols := []string{"Name", "cpu", "ephemeral-storage", "memory", "Window"}
	if !reflect.DeepEqual(cols, wantCols) {
		t.Fatalf("columns = %v, want %v", cols, wantCols)
	}
	if got := table.Rows[0].Cells[2]; got != "" {
		t.Errorf("pod without ephemeral-storage must be blank, got %q", got)
	}
	if got := table.Rows[1].Cells[2]; got != "4Ki" {
		t.Errorf("pod with ephemeral-storage = %q, want 4Ki", got)
	}
}
