// Copyright 2018 The Kubernetes Authors.
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

package sources_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"

	. "github.com/kubernetes-incubator/metrics-server/pkg/sources"
	fakesrc "github.com/kubernetes-incubator/metrics-server/pkg/sources/fake"
)

const timeDrift = 50 * time.Millisecond

func TestSourceManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Source Manager Suite")
}

// sleepySource returns a MetricSource that takes some amount of time (respecting
// context timeouts) to collect a MetricsBatch with a single node's data point.
func sleepySource(delay time.Duration, nodeName string, point MetricsPoint) MetricSource {
	return &fakesrc.FunctionSource{
		SourceName: "sleepy_source:" + nodeName,
		GenerateBatch: func(ctx context.Context) (*MetricsBatch, error) {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, fmt.Errorf("timed out")
			}
			return &MetricsBatch{
				Nodes: []NodeMetricsPoint{
					{
						Name:         nodeName,
						MetricsPoint: point,
					},
				},
			}, nil
		},
	}
}

func fullSource(ts time.Time, nodeInd, podStartInd, numPods int) MetricSource {
	return &fakesrc.FunctionSource{
		SourceName: fmt.Sprintf("static_source:node%d", nodeInd),
		GenerateBatch: func(_ context.Context) (*MetricsBatch, error) {
			podPoints := make([]PodMetricsPoint, numPods)
			for i := range podPoints {
				podInd := int64(podStartInd + i)
				podPoints[i].Name = fmt.Sprintf("pod%d", podInd)
				podPoints[i].Namespace = fmt.Sprintf("ns%d", nodeInd)
				podPoints[i].Containers = []ContainerMetricsPoint{
					{
						Name: "container1",
						MetricsPoint: MetricsPoint{
							Timestamp:   ts,
							CpuUsage:    *resource.NewQuantity(300+10*podInd, resource.DecimalSI),
							MemoryUsage: *resource.NewQuantity(400+10*podInd, resource.DecimalSI),
						},
					},
					{
						Name: "container2",
						MetricsPoint: MetricsPoint{
							Timestamp:   ts,
							CpuUsage:    *resource.NewQuantity(500+10*podInd, resource.DecimalSI),
							MemoryUsage: *resource.NewQuantity(600+10*podInd, resource.DecimalSI),
						},
					},
				}
			}
			return &MetricsBatch{
				Nodes: []NodeMetricsPoint{
					{
						Name: fmt.Sprintf("node%d", nodeInd),
						MetricsPoint: MetricsPoint{
							Timestamp:   ts,
							CpuUsage:    *resource.NewQuantity(100+10*int64(nodeInd), resource.DecimalSI),
							MemoryUsage: *resource.NewQuantity(200+10*int64(nodeInd), resource.DecimalSI),
						},
					},
				},
				Pods: podPoints,
			}, nil
		},
	}
}

var _ = Describe("Source Manager", func() {
	var (
		scrapeTime    = time.Now()
		nodeDataPoint = MetricsPoint{
			Timestamp:   scrapeTime,
			CpuUsage:    *resource.NewQuantity(100, resource.DecimalSI),
			MemoryUsage: *resource.NewQuantity(200, resource.DecimalSI),
		}
	)

	Context("when all sources return in time", func() {
		It("should return the results of all sources, both pods and nodes", func() {
			By("setting up sources that take 1 second to complete")
			metricsSourceProvider := fakesrc.StaticSourceProvider{
				sleepySource(1*time.Second, "node1", nodeDataPoint),
				sleepySource(1*time.Second, "node2", nodeDataPoint),
			}

			By("running the source manager with a scrape and context timeout of 3*seconds")
			start := time.Now()
			manager := NewSourceManager(metricsSourceProvider, 3*time.Second)
			timeoutCtx, doneWithWork := context.WithTimeout(context.Background(), 3*time.Second)
			dataBatch, errs := manager.Collect(timeoutCtx)
			doneWithWork()
			Expect(errs).NotTo(HaveOccurred())

			By("ensuring that the full time took at most 3 seconds")
			Expect(time.Now().Sub(start)).To(BeNumerically("<=", 3*time.Second))

			By("ensuring that all the nodes are listed")
			Expect(dataBatch.Nodes).To(ConsistOf(
				NodeMetricsPoint{Name: "node1", MetricsPoint: nodeDataPoint},
				NodeMetricsPoint{Name: "node2", MetricsPoint: nodeDataPoint},
			))
		})

		It("should return the results of all sources' nodes and pods", func() {
			By("setting up multiple sources")
			metricsSourceProvider := fakesrc.StaticSourceProvider{
				fullSource(scrapeTime, 1, 0, 4),
				fullSource(scrapeTime, 2, 4, 2),
				fullSource(scrapeTime, 3, 6, 1),
			}

			By("running the source manager")
			manager := NewSourceManager(metricsSourceProvider, 1*time.Second)
			dataBatch, errs := manager.Collect(context.Background())
			Expect(errs).NotTo(HaveOccurred())

			By("figuring out the expected node and pod points")
			var expectedNodePoints []interface{}
			var expectedPodPoints []interface{}
			for _, src := range metricsSourceProvider {
				res, err := src.Collect(context.Background())
				Expect(err).NotTo(HaveOccurred())
				for _, pt := range res.Nodes {
					expectedNodePoints = append(expectedNodePoints, pt)
				}
				for _, pt := range res.Pods {
					expectedPodPoints = append(expectedPodPoints, pt)
				}
			}

			By("ensuring that all nodes are present")
			Expect(dataBatch.Nodes).To(ConsistOf(expectedNodePoints...))

			By("ensuring that all pods are present")
			Expect(dataBatch.Pods).To(ConsistOf(expectedPodPoints...))
		})
	})

	Context("when some sources take too long", func() {
		It("should pass the scrape timeout to the source context, so that sources can time out", func() {
			By("setting up one source to take 4 seconds, and another to take 2")
			metricsSourceProvider := fakesrc.StaticSourceProvider{
				sleepySource(4*time.Second, "node1", nodeDataPoint),
				sleepySource(2*time.Second, "node2", nodeDataPoint),
			}

			By("running the source manager with a scrape timeout of 3 seconds")
			start := time.Now()
			manager := NewSourceManager(metricsSourceProvider, 3*time.Second)
			dataBatch, errs := manager.Collect(context.Background())

			By("ensuring that scraping took around 3 seconds")
			Expect(time.Now().Sub(start)).To(BeNumerically("~", 3*time.Second, timeDrift))

			By("ensuring that an error and partial results (data from source 2) were returned")
			Expect(errs).To(HaveOccurred())
			Expect(dataBatch.Nodes).To(ConsistOf(
				NodeMetricsPoint{Name: "node2", MetricsPoint: nodeDataPoint},
			))
		})

		It("should respect the parent context's general timeout, even with a longer scrape timeout", func() {
			By("setting up some sources with 4 second delays")
			metricsSourceProvider := fakesrc.StaticSourceProvider{
				sleepySource(4*time.Second, "node1", nodeDataPoint),
				sleepySource(4*time.Second, "node2", nodeDataPoint),
			}

			By("running the source manager with a scrape timeout of 5 seconds, but a context timeout of 1 second")
			start := time.Now()
			manager := NewSourceManager(metricsSourceProvider, 5*time.Second)
			timeoutCtx, doneWithWork := context.WithTimeout(context.Background(), 1*time.Second)
			dataBatch, errs := manager.Collect(timeoutCtx)
			doneWithWork()

			By("ensuring that it times out after 1 second with errors and no data")
			Expect(time.Now().Sub(start)).To(BeNumerically("~", 1*time.Second, timeDrift))
			Expect(errs).To(HaveOccurred())
			Expect(dataBatch.Nodes).To(BeEmpty())
		})
	})
})
