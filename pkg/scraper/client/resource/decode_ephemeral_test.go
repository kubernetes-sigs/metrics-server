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

package resource

import (
	"testing"
	"time"

	apitypes "k8s.io/apimachinery/pkg/types"
)

const ephBasePayload = `node_cpu_usage_seconds_total 10
node_memory_working_set_bytes 1000
container_cpu_usage_seconds_total{container="c1",namespace="ns1",pod="p1"} 1
container_memory_working_set_bytes{container="c1",namespace="ns1",pod="p1"} 2048
container_start_time_seconds{container="c1",namespace="ns1",pod="p1"} 1
`

func TestDecodeBatchEphemeralStorage(t *testing.T) {
	ref := apitypes.NamespacedName{Namespace: "ns1", Name: "p1"}
	tests := []struct {
		name    string
		payload string
		want    *uint64
	}{
		{
			name:    "present",
			payload: ephBasePayload + `container_ephemeral_storage_usage_bytes{container="c1",namespace="ns1",pod="p1"} 4096` + "\n",
			want:    ephUint64(4096),
		},
		{
			name:    "absent keeps cpu and memory, no false zero",
			payload: ephBasePayload,
			want:    nil,
		},
		{
			name:    "explicit zero is preserved",
			payload: ephBasePayload + `container_ephemeral_storage_usage_bytes{container="c1",namespace="ns1",pod="p1"} 0` + "\n",
			want:    ephUint64(0),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			batch, err := decodeBatch([]byte(tc.payload), time.Unix(1700000000, 0), "node1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			pod, ok := batch.Pods[ref]
			if !ok {
				t.Fatalf("pod %v missing from batch", ref)
			}
			c, ok := pod.Containers["c1"]
			if !ok {
				t.Fatalf("container c1 missing")
			}
			if c.CumulativeCPUUsed == 0 || c.MemoryUsage != 2048 {
				t.Errorf("cpu/memory lost: %+v", c)
			}
			switch {
			case tc.want == nil && c.EphemeralStorageUsage != nil:
				t.Errorf("expected nil ephemeral storage, got %d", *c.EphemeralStorageUsage)
			case tc.want != nil && c.EphemeralStorageUsage == nil:
				t.Errorf("expected %d, got nil", *tc.want)
			case tc.want != nil && *c.EphemeralStorageUsage != *tc.want:
				t.Errorf("expected %d, got %d", *tc.want, *c.EphemeralStorageUsage)
			}
		})
	}
}

func ephUint64(v uint64) *uint64 { return &v }
