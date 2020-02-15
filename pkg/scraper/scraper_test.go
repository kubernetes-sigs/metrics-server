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
	"math"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corelisters "k8s.io/client-go/listers/core/v1"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
	sources "sigs.k8s.io/metrics-server/pkg/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"

	. "sigs.k8s.io/metrics-server/pkg/sources"
	fakesrc "sigs.k8s.io/metrics-server/pkg/sources/fake"
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
			Expect(time.Since(start)).To(BeNumerically("<=", 3*time.Second))

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
			Expect(time.Since(start)).To(BeNumerically("~", 3*time.Second, timeDrift))

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
			Expect(time.Since(start)).To(BeNumerically("~", 1*time.Second, timeDrift))
			Expect(errs).To(HaveOccurred())
			Expect(dataBatch.Nodes).To(BeEmpty())
		})
	})
})

type fakeKubeletClient struct {
	delay   time.Duration
	metrics *stats.Summary

	lastHost string
}

func (c *fakeKubeletClient) GetSummary(ctx context.Context, host string) (*stats.Summary, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("timed out")
	case <-time.After(c.delay):
	}

	c.lastHost = host

	return c.metrics, nil
}

func cpuStats(usageNanocores uint64, ts time.Time) *stats.CPUStats {
	return &stats.CPUStats{
		Time:           metav1.Time{Time: ts},
		UsageNanoCores: &usageNanocores,
	}
}

func memStats(workingSetBytes uint64, ts time.Time) *stats.MemoryStats {
	return &stats.MemoryStats{
		Time:            metav1.Time{Time: ts},
		WorkingSetBytes: &workingSetBytes,
	}
}

func podStats(namespace, name string, containers ...stats.ContainerStats) stats.PodStats {
	return stats.PodStats{
		PodRef: stats.PodReference{
			Name:      name,
			Namespace: namespace,
		},
		Containers: containers,
	}
}

func containerStats(name string, cpu, mem uint64, baseTime time.Time) stats.ContainerStats {
	return stats.ContainerStats{
		Name:   name,
		CPU:    cpuStats(cpu, baseTime.Add(2*time.Millisecond)),
		Memory: memStats(mem, baseTime.Add(4*time.Millisecond)),
	}
}

func verifyNode(nodeName string, summary *stats.Summary, batch *sources.MetricsBatch) {
	var cpuUsage, memoryUsage resource.Quantity
	var timestamp time.Time
	if summary.Node.CPU != nil {
		if summary.Node.CPU.UsageNanoCores != nil {
			cpuUsage = *resource.NewScaledQuantity(int64(*summary.Node.CPU.UsageNanoCores), -9)
		}
		timestamp = summary.Node.CPU.Time.Time
	}
	if summary.Node.Memory != nil {
		if summary.Node.Memory.WorkingSetBytes != nil {
			memoryUsage = *resource.NewQuantity(int64(*summary.Node.Memory.WorkingSetBytes), resource.BinarySI)
		}
		if timestamp.IsZero() {
			timestamp = summary.Node.Memory.Time.Time
		}
	}

	Expect(batch.Nodes).To(ConsistOf(
		sources.NodeMetricsPoint{
			Name: nodeName,
			MetricsPoint: sources.MetricsPoint{
				Timestamp:   timestamp,
				CpuUsage:    cpuUsage,
				MemoryUsage: memoryUsage,
			},
		},
	))
}

func verifyPods(summary *stats.Summary, batch *sources.MetricsBatch) {
	var expectedPods []interface{}
	for _, pod := range summary.Pods {
		containers := make([]sources.ContainerMetricsPoint, len(pod.Containers))
		missingData := false
		for i, container := range pod.Containers {
			var cpuUsage, memoryUsage resource.Quantity
			var timestamp time.Time
			if container.CPU == nil || container.CPU.UsageNanoCores == nil {
				missingData = true
				break
			}
			cpuUsage = *resource.NewScaledQuantity(int64(*container.CPU.UsageNanoCores), -9)
			timestamp = container.CPU.Time.Time
			if container.Memory == nil || container.Memory.WorkingSetBytes == nil {
				missingData = true
				break
			}
			memoryUsage = *resource.NewQuantity(int64(*container.Memory.WorkingSetBytes), resource.BinarySI)
			if timestamp.IsZero() {
				timestamp = container.Memory.Time.Time
			}

			containers[i] = sources.ContainerMetricsPoint{
				Name: container.Name,
				MetricsPoint: sources.MetricsPoint{
					Timestamp:   timestamp,
					CpuUsage:    cpuUsage,
					MemoryUsage: memoryUsage,
				},
			}
		}
		if missingData {
			continue
		}
		expectedPods = append(expectedPods, sources.PodMetricsPoint{
			Name:       pod.PodRef.Name,
			Namespace:  pod.PodRef.Namespace,
			Containers: containers,
		})
	}
	Expect(batch.Pods).To(ConsistOf(expectedPods...))
}

var _ = Describe("Summary Source", func() {
	var (
		src        sources.MetricSource
		client     *fakeKubeletClient
		scrapeTime time.Time = time.Now()
		nodeInfo   NodeInfo  = NodeInfo{
			ConnectAddress: "10.0.1.2",
			Name:           "node1",
		}
	)
	BeforeEach(func() {
		client = &fakeKubeletClient{
			metrics: &stats.Summary{
				Node: stats.NodeStats{
					CPU:    cpuStats(100, scrapeTime.Add(100*time.Millisecond)),
					Memory: memStats(200, scrapeTime.Add(200*time.Millisecond)),
				},
				Pods: []stats.PodStats{
					podStats("ns1", "pod1",
						containerStats("container1", 300, 400, scrapeTime.Add(10*time.Millisecond)),
						containerStats("container2", 500, 600, scrapeTime.Add(20*time.Millisecond))),
					podStats("ns1", "pod2",
						containerStats("container1", 700, 800, scrapeTime.Add(30*time.Millisecond))),
					podStats("ns2", "pod1",
						containerStats("container1", 900, 1000, scrapeTime.Add(40*time.Millisecond))),
					podStats("ns3", "pod1",
						containerStats("container1", 1100, 1200, scrapeTime.Add(50*time.Millisecond))),
				},
			},
		}
		src = NewSummaryMetricsSource(nodeInfo, client)
	})

	It("should pass the provided context to the kubelet client to time out requests", func() {
		By("setting up a context with a 1 second timeout")
		ctx, workDone := context.WithTimeout(context.Background(), 1*time.Second)

		By("collecting the batch with a 4 second delay")
		start := time.Now()
		client.delay = 4 * time.Second
		_, err := src.Collect(ctx)
		workDone()

		By("ensuring it timed out with an error after 1 second")
		Expect(time.Since(start)).To(BeNumerically("~", 1*time.Second, 1*time.Millisecond))
		Expect(err).To(HaveOccurred())
	})

	It("should fetch by connection address", func() {
		By("collecting the batch")
		_, err := src.Collect(context.Background())
		Expect(err).NotTo(HaveOccurred())

		By("verifying that it submitted the right host to the client")
		Expect(client.lastHost).To(Equal(nodeInfo.ConnectAddress))
	})

	It("should return the working set and cpu usage for the node, and all pods on the node", func() {
		By("collecting the batch")
		batch, err := src.Collect(context.Background())
		Expect(err).NotTo(HaveOccurred())

		By("verifying that the batch contains the right node data")
		verifyNode(nodeInfo.Name, client.metrics, batch)

		By("verifying that the batch contains the right pod data")
		verifyPods(client.metrics, batch)
	})

	It("should use the scrape time from the CPU, falling back to memory if missing", func() {
		By("removing some times from the data")
		client.metrics.Pods[0].Containers[0].CPU.Time = metav1.Time{}
		client.metrics.Node.CPU.Time = metav1.Time{}

		By("collecting the batch")
		batch, err := src.Collect(context.Background())
		Expect(err).NotTo(HaveOccurred())

		By("verifying that the scrape time is as expected")
		Expect(batch.Nodes[0].Timestamp).To(Equal(client.metrics.Node.Memory.Time.Time))
		Expect(batch.Pods[0].Containers[0].Timestamp).To(Equal(client.metrics.Pods[0].Containers[0].Memory.Time.Time))
		Expect(batch.Pods[1].Containers[0].Timestamp).To(Equal(client.metrics.Pods[1].Containers[0].CPU.Time.Time))
	})

	It("should continue on missing CPU or memory metrics", func() {
		By("removing some data from the raw summary")
		client.metrics.Node.Memory = nil
		client.metrics.Pods[0].Containers[1].CPU = nil
		client.metrics.Pods[1].Containers[0].CPU.UsageNanoCores = nil
		client.metrics.Pods[2].Containers[0].Memory = nil
		client.metrics.Pods[3].Containers[0].Memory.WorkingSetBytes = nil

		By("collecting the batch")
		batch, err := src.Collect(context.Background())
		Expect(err).To(HaveOccurred())

		By("verifying that the batch has all the data, save for what was missing")
		verifyNode(nodeInfo.Name, client.metrics, batch)
		verifyPods(client.metrics, batch)
	})

	It("should handle larger-than-int64 CPU or memory values gracefully", func() {
		By("setting some data in the summary to be above math.MaxInt64")
		plusTen := uint64(math.MaxInt64 + 10)
		plusTwenty := uint64(math.MaxInt64 + 20)
		minusTen := uint64(math.MaxUint64 - 10)
		minusOneHundred := uint64(math.MaxUint64 - 100)

		client.metrics.Node.Memory.WorkingSetBytes = &plusTen // RAM is cheap, right?
		client.metrics.Node.CPU.UsageNanoCores = &plusTwenty  // a mainframe, probably
		client.metrics.Pods[0].Containers[1].CPU.UsageNanoCores = &minusTen
		client.metrics.Pods[1].Containers[0].Memory.WorkingSetBytes = &minusOneHundred

		By("collecting the batch")
		batch, err := src.Collect(context.Background())
		Expect(err).NotTo(HaveOccurred())

		By("verifying that the data is still present, at lower precision")
		nodeMem := *resource.NewScaledQuantity(int64(plusTen/10), 1)
		nodeMem.Format = resource.BinarySI
		podMem := *resource.NewScaledQuantity(int64(minusOneHundred/10), 1)
		podMem.Format = resource.BinarySI
		Expect(batch.Nodes[0].MemoryUsage).To(Equal(nodeMem))
		Expect(batch.Nodes[0].CpuUsage).To(Equal(*resource.NewScaledQuantity(int64(plusTwenty/10), -8)))
		Expect(batch.Pods[0].Containers[1].CpuUsage).To(Equal(*resource.NewScaledQuantity(int64(minusTen/10), -8)))
		Expect(batch.Pods[1].Containers[0].MemoryUsage).To(Equal(podMem))
	})
})

type fakeNodeLister struct {
	nodes   []*corev1.Node
	listErr error
}

func (l *fakeNodeLister) List(_ labels.Selector) (ret []*corev1.Node, err error) {
	if l.listErr != nil {
		return nil, l.listErr
	}
	// NB: this is ignores selector for the moment
	return l.nodes, nil
}

func (l *fakeNodeLister) ListWithPredicate(_ corelisters.NodeConditionPredicate) ([]*corev1.Node, error) {
	// NB: this is ignores predicate for the moment
	return l.List(labels.Everything())
}

func (l *fakeNodeLister) Get(name string) (*corev1.Node, error) {
	for _, node := range l.nodes {
		if node.Name == name {
			return node, nil
		}
	}
	return nil, fmt.Errorf("no such node %q", name)
}

func nodeNames(nodes []*corev1.Node, addrs []string) []string {
	var res []string
	for i, node := range nodes {
		res = append(res, NewSummaryMetricsSource(NodeInfo{ConnectAddress: addrs[i], Name: node.Name}, nil).Name())
	}
	return res
}

func makeNode(name, hostName, addr string, ready bool) *corev1.Node {
	res := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node1"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeHostName, Address: hostName},
				{Type: corev1.NodeInternalIP, Address: addr},
			},
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady},
			},
		},
	}
	if ready {
		res.Status.Conditions[0].Status = corev1.ConditionTrue
	} else {
		res.Status.Conditions[0].Status = corev1.ConditionFalse
	}
	return res
}

var _ = Describe("Summary Source Provider", func() {
	var (
		nodeLister *fakeNodeLister
		nodeAddrs  []string
		provider   sources.MetricSourceProvider
		fakeClient *fakeKubeletClient
	)
	BeforeEach(func() {
		nodeLister = &fakeNodeLister{
			nodes: []*corev1.Node{
				makeNode("node1", "node1.somedomain", "10.0.1.2", true),
				makeNode("node-no-host", "", "10.0.1.3", true),
				makeNode("node3", "node3.somedomain", "10.0.1.4", false),
				makeNode("node4", "node4.somedomain", "10.0.1.5", true),
			},
		}
		nodeAddrs = []string{
			"10.0.1.2",
			"10.0.1.3",
			"10.0.1.4",
			"10.0.1.5",
		}
		fakeClient = &fakeKubeletClient{}
		addrResolver := NewPriorityNodeAddressResolver(DefaultAddressTypePriority)
		provider = NewSummaryProvider(nodeLister, fakeClient, addrResolver)
	})

	It("should return a metrics source for all nodes", func() {
		By("listing the sources")
		sources, err := provider.GetMetricSources()
		Expect(err).To(Succeed())

		By("verifying that a source is present for each node")
		nodeNames := nodeNames(nodeLister.nodes, nodeAddrs)
		sourceNames := make([]string, len(nodeNames))
		for i, src := range sources {
			sourceNames[i] = src.Name()
		}
		Expect(sourceNames).To(Equal(nodeNames))
	})

	It("should continue on error fetching node information for a particular node", func() {
		By("deleting the IP of a node")
		nodeLister.nodes[0].Status.Addresses = nil

		By("listing the sources")
		sources, err := provider.GetMetricSources()
		Expect(err).To(HaveOccurred())

		By("verifying that a source is present for each node")
		nodeNames := nodeNames(nodeLister.nodes, nodeAddrs)
		sourceNames := make([]string, len(nodeNames[1:]))
		for i, src := range sources {
			sourceNames[i] = src.Name()
		}
		// skip the bad node (the first one)
		Expect(sourceNames).To(Equal(nodeNames[1:]))
	})

	It("should gracefully handle list errors", func() {
		By("setting a fake error from the lister")
		nodeLister.listErr = fmt.Errorf("something went wrong, expectedly")

		By("listing the sources")
		_, err := provider.GetMetricSources()
		Expect(err).To(HaveOccurred())
	})

	Describe("when choosing node addresses", func() {
		JustBeforeEach(func() {
			// set up the metrics so we can call collect safely
			fakeClient.metrics = &stats.Summary{
				Node: stats.NodeStats{
					CPU:    cpuStats(100, time.Now()),
					Memory: memStats(200, time.Now()),
				},
			}
		})

		It("should prefer addresses according to the order of the types first", func() {
			By("setting the first node to have multiple addresses and setting all nodes to ready")
			nodeLister.nodes[0].Status.Addresses = []corev1.NodeAddress{
				{Type: DefaultAddressTypePriority[3], Address: "skip-val1"},
				{Type: DefaultAddressTypePriority[2], Address: "skip-val2"},
				{Type: DefaultAddressTypePriority[1], Address: "correct-val"},
			}
			for _, node := range nodeLister.nodes {
				node.Status.Conditions = []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				}
			}

			By("listing all sources")
			srcs, err := provider.GetMetricSources()
			Expect(err).NotTo(HaveOccurred())

			By("making sure that the first source scrapes from the correct location")
			_, err = srcs[0].Collect(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeClient.lastHost).To(Equal("correct-val"))
		})

		It("should prefer the first address that matches within a given type", func() {
			By("setting the first node to have multiple addresses and setting all nodes to ready")
			nodeLister.nodes[0].Status.Addresses = []corev1.NodeAddress{
				{Type: DefaultAddressTypePriority[1], Address: "skip-val1"},
				{Type: DefaultAddressTypePriority[0], Address: "correct-val"},
				{Type: DefaultAddressTypePriority[1], Address: "skip-val2"},
				{Type: DefaultAddressTypePriority[0], Address: "second-val"},
			}
			for _, node := range nodeLister.nodes {
				node.Status.Conditions = []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				}
			}

			By("listing all sources")
			srcs, err := provider.GetMetricSources()
			Expect(err).NotTo(HaveOccurred())

			By("making sure that the first source scrapes from the correct location")
			_, err = srcs[0].Collect(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeClient.lastHost).To(Equal("correct-val"))
		})

		It("should return an error if no preferred addresses are found", func() {
			By("wiping out the addresses of one of the nodes and setting all nodes to ready")
			nodeLister.nodes[0].Status.Addresses = nil
			for _, node := range nodeLister.nodes {
				node.Status.Conditions = []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				}
			}

			By("asking for source providers for all nodes")
			_, err := provider.GetMetricSources()
			Expect(err).To(HaveOccurred())
		})
	})
})
