// Copyright 2020 The Kubernetes Authors.
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

package summary

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/google/go-cmp/cmp"
	"github.com/mailru/easyjson"

	"k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"sigs.k8s.io/metrics-server/pkg/storage"
)

func TestTypes(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Decode Suite")
}

var _ = Describe("Types", func() {
	It("internal Summary should be compatible with stats.Summary", func() {
		By("Unmarshaling json into stats.Summary")
		stats := &v1alpha1.Summary{}
		err := json.Unmarshal([]byte(data), stats)
		Expect(err).NotTo(HaveOccurred())

		By("Unmarshaling json into internal Summary")
		internal := &Summary{}
		err = json.Unmarshal([]byte(data), internal)
		Expect(err).NotTo(HaveOccurred())

		By("Comparing values")
		err = compare(stats, internal)
		Expect(err).NotTo(HaveOccurred())
	})

	It("internal summary should include all values needed", func() {
		By("Unmarshaling json into internal Summary")
		internal := &Summary{}
		err := easyjson.Unmarshal([]byte(data), internal)
		Expect(err).NotTo(HaveOccurred())

		By("checking decoded metrics match expected")
		got := decodeBatch(internal)
		diff := cmp.Diff(got, expected)
		Expect(diff).To(BeEmpty())
	})
})

func compare(stats *v1alpha1.Summary, internal *Summary) error {
	if len(internal.Pods) != len(internal.Pods) {
		return fmt.Errorf("diff: len(.Pods)")
	}
	for i := range stats.Pods {
		if internal.Pods[i].PodRef.Name != stats.Pods[i].PodRef.Name {
			return fmt.Errorf("diff: .Pods[%d].PodRef.Name ", i)
		}
		if internal.Pods[i].PodRef.Namespace != stats.Pods[i].PodRef.Namespace {
			return fmt.Errorf("diff: stats.Pods[%d].Namespace", i)
		}
		if len(internal.Pods[i].Containers) != len(stats.Pods[i].Containers) {
			return fmt.Errorf("diff: len(stats.Pods[%d].Containers)", i)
		}
		for j := range internal.Pods[i].Containers {
			if internal.Pods[i].Containers[j].Name != stats.Pods[i].Containers[j].Name {
				return fmt.Errorf("diff: stats.Pods[%d].Containers[%d].Name", i, j)
			}
			err := compareCPU(stats.Pods[i].Containers[j].CPU, internal.Pods[i].Containers[j].CPU)
			if err != nil {
				return fmt.Errorf("diff: stats.Pods[%d].Containers[%d].CPU%v", i, j, err)
			}
			err = compareMemory(stats.Pods[i].Containers[j].Memory, internal.Pods[i].Containers[j].Memory)
			if err != nil {
				return fmt.Errorf("diff: stats.Pods[%d].Containers[%d].Memory%v", i, j, err)
			}
		}
	}
	if internal.Node.NodeName != stats.Node.NodeName {
		return fmt.Errorf("diff: .Node.NodeName")
	}
	err := compareCPU(stats.Node.CPU, internal.Node.CPU)
	if err != nil {
		return fmt.Errorf("diff: .Node.CPU%v", err)
	}
	err = compareMemory(stats.Node.Memory, internal.Node.Memory)
	if err != nil {
		return fmt.Errorf("diff: .Node.Memory%v", err)
	}
	return nil
}

func compareCPU(stats *v1alpha1.CPUStats, internal *CPUStats) error {
	if (stats == nil) != (internal == nil) {
		return fmt.Errorf("== nil")
	}
	if stats == nil || internal == nil {
		return nil
	}
	if internal.Time != stats.Time {
		return fmt.Errorf(".Time")
	}
	if (internal.UsageCoreNanoSeconds == nil) != (stats.UsageCoreNanoSeconds == nil) {
		return fmt.Errorf(".UsageCoreNanoSeconds")
	}
	if (internal.UsageCoreNanoSeconds == nil) || (stats.UsageCoreNanoSeconds == nil) {
		return nil
	}
	if *internal.UsageCoreNanoSeconds != *stats.UsageCoreNanoSeconds {
		return fmt.Errorf(".UsageCoreNanoSeconds")
	}
	return nil
}

func compareMemory(stats *v1alpha1.MemoryStats, internal *MemoryStats) error {
	if (stats == nil) != (internal == nil) {
		return fmt.Errorf("== nil")
	}
	if stats == nil || internal == nil {
		return nil
	}
	if internal.Time != stats.Time {
		return fmt.Errorf(".Time")
	}
	if (internal.WorkingSetBytes == nil) != (stats.WorkingSetBytes == nil) {
		return fmt.Errorf(".WorkingSetBytes")
	}
	if (internal.WorkingSetBytes == nil) || (stats.WorkingSetBytes == nil) {
		return nil
	}
	if *internal.WorkingSetBytes != *stats.WorkingSetBytes {
		return fmt.Errorf(".WorkingSetBytes")
	}
	return nil
}

func BenchmarkJSONUnmarshal(b *testing.B) {
	value := &Summary{}
	for i := 0; i < b.N; i++ {
		err := easyjson.Unmarshal([]byte(data), value)
		if err != nil {
			b.Error(err)
		}
	}
}

var data = `
{
 "node": {
  "nodeName": "e2e-v1.17.0-control-plane",
  "systemContainers": [
   {
    "name": "kubelet",
    "startTime": "2020-04-16T20:05:46Z",
    "cpu": {
     "time": "2020-04-16T20:25:30Z",
     "usageNanoCores": 287620424,
     "usageCoreNanoSeconds": 183912297212
    },
    "memory": {
     "time": "2020-04-16T20:25:30Z",
     "usageBytes": 146317312,
     "workingSetBytes": 122638336,
     "rssBytes": 85635072,
     "pageFaults": 1757976,
     "majorPageFaults": 528
    }
   },
   {
    "name": "pods",
    "startTime": "2020-04-16T20:21:41Z",
    "cpu": {
     "time": "2020-04-16T20:25:28Z",
     "usageNanoCores": 165934426,
     "usageCoreNanoSeconds": 231508341412
    },
    "memory": {
     "time": "2020-04-16T20:25:28Z",
     "availableBytes": 15915753472,
     "usageBytes": 752480256,
     "workingSetBytes": 713609216,
     "rssBytes": 381231104,
     "pageFaults": 0,
     "majorPageFaults": 0
    }
   }
  ],
  "startTime": "2020-03-31T18:00:54Z",
  "cpu": {
   "time": "2020-04-16T20:25:28Z",
   "usageNanoCores": 476553087,
   "usageCoreNanoSeconds": 519978197128
  },
  "memory": {
   "time": "2020-04-16T20:25:28Z",
   "availableBytes": 15211810816,
   "usageBytes": 1719095296,
   "workingSetBytes": 1417551872,
   "rssBytes": 848789504,
   "pageFaults": 73326,
   "majorPageFaults": 726
  },
  "network": {
   "time": "2020-04-16T20:25:28Z",
   "name": "eth0",
   "rxBytes": 9848384,
   "rxErrors": 0,
   "txBytes": 72810891,
   "txErrors": 0,
   "interfaces": [
    {
     "name": "eth0",
     "rxBytes": 9848384,
     "rxErrors": 0,
     "txBytes": 72810891,
     "txErrors": 0
    }
   ]
  },
  "fs": {
   "time": "2020-04-16T20:25:28Z",
   "availableBytes": 366430162944,
   "capacityBytes": 500684595200,
   "usedBytes": 108749709312,
   "inodesFree": 29713960,
   "inodes": 31113216,
   "inodesUsed": 1399256
  },
  "runtime": {
   "imageFs": {
    "time": "2020-04-16T20:25:26Z",
    "availableBytes": 366430162944,
    "capacityBytes": 500684595200,
    "usedBytes": 789861024,
    "inodesFree": 29713960,
    "inodes": 31113216,
    "inodesUsed": 8769
   }
  },
  "rlimit": {
   "time": "2020-04-16T20:25:30Z",
   "maxpid": 32768,
   "curproc": 3317
  }
 },
 "pods": [
  {
   "podRef": {
    "name": "all-fields",
    "namespace": "default",
    "uid": "96636a87-47f5-4970-a15e-6e7901925c90"
   },
   "startTime": "2020-04-16T20:11:06Z",
   "containers": [
    {
     "name": "container",
     "startTime": "2020-04-16T20:17:46Z",
     "cpu": {
      "time": "2020-04-16T20:25:30Z",
      "usageNanoCores": 29713960,
      "usageCoreNanoSeconds": 29328792
     },
     "memory": {
      "time": "2020-04-16T20:25:30Z",
      "workingSetBytes": 1449984
     },
     "rootfs": {
      "time": "2020-04-16T20:25:26Z",
      "availableBytes": 366430162944,
      "capacityBytes": 500684595200,
      "usedBytes": 24576,
      "inodesFree": 29713960,
      "inodes": 31113216,
      "inodesUsed": 7
     },
     "logs": {
      "time": "2020-04-16T20:25:30Z",
      "availableBytes": 366430162944,
      "capacityBytes": 500684595200,
      "usedBytes": 4096,
      "inodesFree": 29713960,
      "inodes": 31113216,
      "inodesUsed": 2
     }
    }
   ],
   "cpu": {
    "time": "2020-04-16T20:25:24Z",
    "usageNanoCores": 123,
    "usageCoreNanoSeconds": 54096725
   },
   "memory": {
    "time": "2020-04-16T20:25:24Z",
    "usageBytes": 2641920,
    "workingSetBytes": 2641920,
    "rssBytes": 0,
    "pageFaults": 0,
    "majorPageFaults": 0
   },
   "volume": [
    {
     "time": "2020-04-16T20:11:49Z",
     "availableBytes": 8314667008,
     "capacityBytes": 8314679296,
     "usedBytes": 12288,
     "inodesFree": 2029942,
     "inodes": 2029951,
     "inodesUsed": 9,
     "name": "default-token-sd9l8"
    }
   ],
   "ephemeral-storage": {
    "time": "2020-04-16T20:25:30Z",
    "availableBytes": 366430162944,
    "capacityBytes": 500684595200,
    "usedBytes": 28672,
    "inodesFree": 29713960,
    "inodes": 31113216,
    "inodesUsed": 9
   }
  },
  {
   "podRef": {
    "name": "zero usageCoreNanoSeconds",
    "namespace": "default"
   },
   "startTime": "2020-04-16T20:11:06Z",
   "containers": [
    {
     "name": "container",
     "startTime": "2020-04-16T20:17:46Z",
     "cpu": {
      "time": "2020-04-16T20:25:30Z",
      "usageNanoCores": 29713960,
      "usageCoreNanoSeconds": 0
     },
     "memory": {
      "time": "2020-04-16T20:25:30Z",
      "workingSetBytes": 1449984
     }
    }
   ]
  },
  {
   "podRef": {
    "name": "no usageCoreNanoSeconds",
    "namespace": "default"
   },
   "startTime": "2020-04-16T20:11:06Z",
   "containers": [
    {
     "name": "container",
     "startTime": "2020-04-16T20:17:46Z",
     "cpu": {
      "time": "2020-04-16T20:25:30Z",
      "usageNanoCores": 29713960
     },
     "memory": {
      "time": "2020-04-16T20:25:30Z",
      "workingSetBytes": 1449984
     }
    }
   ]
  },
  {
   "podRef": {
    "name": "no CPU",
    "namespace": "default"
   },
   "startTime": "2020-04-16T20:11:06Z",
   "containers": [
    {
     "name": "container",
     "startTime": "2020-04-16T20:17:46Z",
     "memory": {
      "time": "2020-04-16T20:25:30Z",
      "workingSetBytes": 1449984
     }
    }
   ]
  },
  {
   "podRef": {
    "name": "zero workingSetBytes",
    "namespace": "default"
   },
   "startTime": "2020-04-16T20:11:06Z",
   "containers": [
    {
     "name": "container",
     "startTime": "2020-04-16T20:17:46Z",
     "cpu": {
      "time": "2020-04-16T20:25:30Z",
      "usageNanoCores": 29713960,
      "usageCoreNanoSeconds": 29328792
     },
     "memory": {
      "time": "2020-04-16T20:25:30Z",
      "workingSetBytes": 0
     }
    }
   ]
  },
  {
   "podRef": {
    "name": "no workingSetBytes",
    "namespace": "default"
   },
   "startTime": "2020-04-16T20:11:06Z",
   "containers": [
    {
     "name": "container",
     "startTime": "2020-04-16T20:17:46Z",
     "cpu": {
      "time": "2020-04-16T20:25:30Z",
      "usageNanoCores": 29713960,
      "usageCoreNanoSeconds": 29328792
     },
     "memory": {
      "time": "2020-04-16T20:25:30Z"
     }
    }
   ]
  },
  {
   "podRef": {
    "name": "no memory",
    "namespace": "default"
   },
   "startTime": "2020-04-16T20:11:06Z",
   "containers": [
    {
     "name": "container",
     "startTime": "2020-04-16T20:17:46Z",
     "cpu": {
      "time": "2020-04-16T20:25:30Z",
      "usageNanoCores": 29713960,
      "usageCoreNanoSeconds": 29328792
     }
    }
   ]
  }
 ]
}
`

var expected = &storage.MetricsBatch{
	Nodes: []storage.NodeMetricsPoint{
		{
			Name: "e2e-v1.17.0-control-plane",
			MetricsPoint: storage.MetricsPoint{
				StartTime:         time.Date(2020, 3, 31, 18, 00, 54, 0, time.UTC),
				Timestamp:         time.Date(2020, 4, 16, 20, 25, 28, 0, time.UTC),
				CumulativeCpuUsed: 519978197128,
				MemoryUsage:       1417551872,
			},
		},
	},
	Pods: []storage.PodMetricsPoint{
		{
			Name:      "all-fields",
			Namespace: "default",
			Containers: []storage.ContainerMetricsPoint{
				{
					Name: "container",
					MetricsPoint: storage.MetricsPoint{
						StartTime:         time.Date(2020, 4, 16, 20, 17, 46, 0, time.UTC),
						Timestamp:         time.Date(2020, 4, 16, 20, 25, 30, 0, time.UTC),
						CumulativeCpuUsed: 29328792,
						MemoryUsage:       1449984,
					},
				},
			},
		},
	},
}
