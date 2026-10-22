// Copyright The Kubernetes Authors.
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
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func TestSkippedNodeLogsOnlyObjectReference(t *testing.T) {
	state := klog.CaptureState()
	t.Cleanup(state.Restore)
	var output strings.Builder
	klog.ClearLogger()
	klog.LogToStderr(false)
	klog.SetOutput(&output)
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:        "node1",
		Labels:      map[string]string{"private-label": "label-canary"},
		Annotations: map[string]string{"private-annotation": "annotation-canary"},
	}}
	now := time.Now()
	storage := nodeStorage{
		last: map[string]MetricsPoint{"node1": {StartTime: now, CumulativeCPUUsed: 1}},
		prev: map[string]MetricsPoint{"node1": {StartTime: now, CumulativeCPUUsed: 2}},
	}
	metrics, err := storage.GetMetrics(node)
	klog.Flush()
	if err != nil || len(metrics) != 0 {
		t.Fatalf("GetMetrics() = %v, %v; want skipped metrics", metrics, err)
	}
	if !strings.Contains(output.String(), "node1") || !strings.Contains(output.String(), "Skipping node usage metric") {
		t.Fatalf("missing diagnostic context: %s", output.String())
	}
	if strings.Contains(output.String(), "canary") {
		t.Fatalf("node metadata leaked in diagnostic: %s", output.String())
	}
	if node.Annotations["private-annotation"] != "annotation-canary" {
		t.Fatal("input metadata was modified")
	}
}
