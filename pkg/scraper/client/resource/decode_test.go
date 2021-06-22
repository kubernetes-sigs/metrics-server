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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/prometheus/common/model"
)

func TestDecode(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Decode Suite")
}

var _ = Describe("Decode", func() {
	var (
		samples []*model.Sample
	)

	BeforeEach(func() {
		scrapeTime := time.Now()

		sample1 := model.Sample{Metric: model.Metric{model.MetricNameLabel: "node_cpu_usage_seconds_total"},
			Value:     100,
			Timestamp: model.Time(scrapeTime.Add(100*time.Millisecond).UnixNano() / 1e6),
		}
		sample2 := model.Sample{Metric: model.Metric{model.MetricNameLabel: "node_memory_working_set_bytes"},
			Value:     200,
			Timestamp: model.Time(scrapeTime.Add(100*time.Millisecond).UnixNano() / 1e6),
		}
		sample3 := model.Sample{Metric: model.Metric{model.MetricNameLabel: "container_cpu_usage_seconds_total", "container": "container1", "namespace": "ns1", "pod": "pod1"},
			Value:     300,
			Timestamp: model.Time(scrapeTime.Add(10*time.Millisecond).Unix() / 1e6),
		}
		sample4 := model.Sample{Metric: model.Metric{model.MetricNameLabel: "container_memory_working_set_bytes", "container": "container1", "namespace": "ns1", "pod": "pod1"},
			Value:     400,
			Timestamp: model.Time(scrapeTime.Add(10*time.Millisecond).Unix() / 1e6),
		}
		sample5 := model.Sample{Metric: model.Metric{model.MetricNameLabel: "container_cpu_usage_seconds_total", "container": "container2", "namespace": "ns1", "pod": "pod1"},
			Value:     500,
			Timestamp: model.Time(scrapeTime.Add(20*time.Millisecond).Unix() / 1e6),
		}
		sample6 := model.Sample{Metric: model.Metric{model.MetricNameLabel: "container_memory_working_set_bytes", "container": "container2", "namespace": "ns1", "pod": "pod1"},
			Value:     600,
			Timestamp: model.Time(scrapeTime.Add(20*time.Millisecond).Unix() / 1e6),
		}
		sample7 := model.Sample{Metric: model.Metric{model.MetricNameLabel: "container_cpu_usage_seconds_total", "container": "container1", "namespace": "ns1resource", "pod": "pod2"},
			Value:     700,
			Timestamp: model.Time(scrapeTime.Add(30*time.Millisecond).Unix() / 1e6),
		}
		sample8 := model.Sample{Metric: model.Metric{model.MetricNameLabel: "container_memory_working_set_bytes", "container": "container1", "namespace": "ns1resource", "pod": "pod2"},
			Value:     800,
			Timestamp: model.Time(scrapeTime.Add(30*time.Millisecond).Unix() / 1e6),
		}
		sample9 := model.Sample{Metric: model.Metric{model.MetricNameLabel: "container_cpu_usage_seconds_total", "container": "container1", "namespace": "ns2", "pod": "pod1"},
			Value:     900,
			Timestamp: model.Time(scrapeTime.Add(40*time.Millisecond).Unix() / 1e6),
		}
		sample10 := model.Sample{Metric: model.Metric{model.MetricNameLabel: "container_memory_working_set_bytes", "container": "container1", "namespace": "ns2", "pod": "pod1"},
			Value:     1000,
			Timestamp: model.Time(scrapeTime.Add(40*time.Millisecond).Unix() / 1e6),
		}
		sample11 := model.Sample{Metric: model.Metric{model.MetricNameLabel: "container_cpu_usage_seconds_total", "container": "container1", "namespace": "ns3", "pod": "pod1"},
			Value:     1100,
			Timestamp: model.Time(scrapeTime.Add(50*time.Millisecond).Unix() / 1e6),
		}
		sample12 := model.Sample{Metric: model.Metric{model.MetricNameLabel: "container_memory_working_set_bytes", "container": "container1", "namespace": "ns3", "pod": "pod1"},
			Value:     1200,
			Timestamp: model.Time(scrapeTime.Add(50*time.Millisecond).Unix() / 1e6),
		}
		samples = []*model.Sample{}
		samples = append(samples, &sample1, &sample2, &sample3, &sample4, &sample5, &sample6, &sample7, &sample8, &sample9, &sample10, &sample11, &sample12)
	})

	It("should use the decode time from the CPU", func() {
		By("removing some times from the data")

		By("decoding")
		batch := decodeBatch(samples, "node1")

		By("verifying that the scrape time is as expected")
		Expect(batch.Nodes["node1"].Timestamp).To(Equal(time.Unix(0, int64(samples[0].Timestamp*1e6))))
		for _, p := range batch.Pods {
			if p.Namespace == "ns1" && p.Name == "pod1" {
				for _, c := range p.Containers {
					if c.Name == "container1" {
						Expect(c.Timestamp).To(Equal(time.Unix(0, int64(samples[2].Timestamp*1e6))))
					}
				}
			} else if p.Namespace == "ns1resource" && p.Name == "pod2" {
				for _, c := range p.Containers {
					if c.Name == "container1" {
						Expect(c.Timestamp).To(Equal(time.Unix(0, int64(samples[6].Timestamp*1e6))))
					}
				}
			}

		}
	})

	It("should continue on missing CPU or memory metrics", func() {
		By("removing some data from the raw resource")
		samples[1].Timestamp = 0
		samples[1].Value = 0
		samples[4].Timestamp = 0
		samples[4].Value = 0
		samples[6].Value = 0
		samples[9].Timestamp = 0
		samples[9].Value = 0
		samples[11].Value = 0

		By("decoding")
		batch := decodeBatch(samples, "node1")

		By("verifying that the batch has all the data, save for what was missing")
		Expect(batch.Pods).To(HaveLen(0))
		Expect(batch.Nodes).To(HaveLen(0))
	})

	It("should skip on cumulative CPU equal zero", func() {
		By("setting CPU cumulative value to zero")
		samples[0].Value = 0
		samples[2].Value = 0

		By("decoding")
		batch := decodeBatch(samples, "node1")

		By("verifying that zero records were deleted")
		Expect(batch.Pods).To(HaveLen(3))
		Expect(batch.Nodes).To(HaveLen(0))
	})

})
