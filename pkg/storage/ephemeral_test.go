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

package storage

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
)

func ephPtr(v uint64) *uint64 { return &v }

func TestResourceUsageEphemeralStorage(t *testing.T) {
	t0 := time.Unix(1700000000, 0)
	prev := MetricsPoint{StartTime: t0.Add(-time.Hour), Timestamp: t0, CumulativeCPUUsed: 1e9, MemoryUsage: 100}
	mk := func(e *uint64) MetricsPoint {
		return MetricsPoint{StartTime: t0.Add(-time.Hour), Timestamp: t0.Add(10 * time.Second), CumulativeCPUUsed: 2e9, MemoryUsage: 200, EphemeralStorageUsage: e}
	}

	t.Run("present", func(t *testing.T) {
		rl, _, err := resourceUsage(mk(ephPtr(4096)), prev)
		if err != nil {
			t.Fatal(err)
		}
		q, ok := rl[corev1.ResourceEphemeralStorage]
		if !ok {
			t.Fatal("ephemeral-storage missing")
		}
		if q.Value() != 4096 || q.Format != resource.BinarySI {
			t.Errorf("got %v (%v)", q.Value(), q.Format)
		}
	})
	t.Run("absent", func(t *testing.T) {
		rl, _, err := resourceUsage(mk(nil), prev)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := rl[corev1.ResourceEphemeralStorage]; ok {
			t.Error("ephemeral-storage must be omitted when nil")
		}
		if _, ok := rl[corev1.ResourceCPU]; !ok {
			t.Error("cpu missing")
		}
		if _, ok := rl[corev1.ResourceMemory]; !ok {
			t.Error("memory missing")
		}
	})
	t.Run("explicit zero", func(t *testing.T) {
		rl, _, err := resourceUsage(mk(ephPtr(0)), prev)
		if err != nil {
			t.Fatal(err)
		}
		q, ok := rl[corev1.ResourceEphemeralStorage]
		if !ok || !q.IsZero() {
			t.Errorf("expected present zero quantity, got ok=%v q=%v", ok, q)
		}
	})
}

func TestPodStorageEphemeralMixedContainers(t *testing.T) {
	t0 := time.Unix(1700000000, 0)
	start := t0.Add(-time.Hour)
	ref := apitypes.NamespacedName{Namespace: "ns", Name: "pod"}
	batch := func(ts time.Time, cpu uint64) *MetricsBatch {
		return &MetricsBatch{Pods: map[apitypes.NamespacedName]PodMetricsPoint{
			ref: {Containers: map[string]MetricsPoint{
				"with":    {StartTime: start, Timestamp: ts, CumulativeCPUUsed: cpu, MemoryUsage: 10, EphemeralStorageUsage: ephPtr(2048)},
				"without": {StartTime: start, Timestamp: ts, CumulativeCPUUsed: cpu, MemoryUsage: 10},
			}},
		}}
	}
	s := NewStorage(60 * time.Second)
	s.Store(batch(t0, 1e9))
	s.Store(batch(t0.Add(15*time.Second), 2e9))

	got, err := s.GetPodMetrics(&metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "pod"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Containers) != 2 {
		t.Fatalf("unexpected result: %+v", got)
	}
	for _, c := range got[0].Containers {
		_, has := c.Usage[corev1.ResourceEphemeralStorage]
		if c.Name == "with" && !has {
			t.Error("container 'with' should expose ephemeral-storage")
		}
		if c.Name == "without" && has {
			t.Error("container 'without' must not expose ephemeral-storage")
		}
	}
}

