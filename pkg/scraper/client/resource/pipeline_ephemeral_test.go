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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/metrics-server/pkg/storage"
)

func e2ePayload(ts int64, cpu float64, eph uint64) string {
	return fmt.Sprintf(`node_cpu_usage_seconds_total 10 %[1]d
node_memory_working_set_bytes 1000 %[1]d
container_cpu_usage_seconds_total{container="c1",namespace="ns1",pod="p1"} %[2]v %[1]d
container_memory_working_set_bytes{container="c1",namespace="ns1",pod="p1"} 2048 %[1]d
container_start_time_seconds{container="c1",namespace="ns1",pod="p1"} 1
container_ephemeral_storage_usage_bytes{container="c1",namespace="ns1",pod="p1"} %[3]d %[1]d
`, ts, cpu, eph)
}

func TestPipelineEphemeralStorage(t *testing.T) {
	payloads := []string{
		e2ePayload(1700000000000, 1, 4096),
		e2ePayload(1700000015000, 2, 8192),
	}
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, payloads[call])
		call++
	}))
	defer srv.Close()

	kc := newClient(srv.Client(), nil, 0, "http", false)
	st := storage.NewStorage(60 * time.Second)
	for i := 0; i < 2; i++ {
		batch, err := kc.getMetrics(context.Background(), srv.URL, "node1")
		if err != nil {
			t.Fatalf("scrape %d failed: %v", i, err)
		}
		st.Store(batch)
	}

	got, err := st.GetPodMetrics(&metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "p1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Containers) != 1 {
		t.Fatalf("unexpected result: %+v", got)
	}
	usage := got[0].Containers[0].Usage
	for _, name := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory, corev1.ResourceEphemeralStorage} {
		if _, ok := usage[name]; !ok {
			t.Errorf("%s missing from PodMetrics", name)
		}
	}
	eph := usage[corev1.ResourceEphemeralStorage]
	if eph.Value() != 8192 {
		t.Errorf("ephemeral-storage = %d, want 8192 (latest scrape)", eph.Value())
	}
}