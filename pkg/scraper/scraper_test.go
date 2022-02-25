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

package scraper

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/metrics-server/pkg/scraper/client"
	"sigs.k8s.io/metrics-server/pkg/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/component-base/metrics/testutil"
)

const timeDrift = 50 * time.Millisecond

func TestScraper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scraper Suite")
}

var _ = Describe("Scraper", func() {
	var (
		scrapeTime       = time.Now()
		metricResolution = 5 * time.Second
		client           fakeKubeletClient
		node1            = makeNode("node1", "node1.somedomain", "10.0.1.2", true)
		node2            = makeNode("node-no-host", "", "10.0.1.3", true)
		node3            = makeNode("node3", "node3.somedomain", "10.0.1.4", false)
		node4            = makeNode("node4", "node4.somedomain", "10.0.1.5", true)
		store            = storage.NewStorage(metricResolution)
	)
	BeforeEach(func() {
		mb := &storage.MetricsBatch{
			Nodes: map[string]storage.MetricsPoint{
				node1.Name: metricPoint(100, 200, scrapeTime),
			},
			Pods: map[apitypes.NamespacedName]storage.PodMetricsPoint{
				{Namespace: "ns1", Name: "pod1"}: {
					Containers: map[string]storage.MetricsPoint{
						"container1": metricPoint(300, 400, scrapeTime.Add(10*time.Millisecond)),
						"container2": metricPoint(500, 600, scrapeTime.Add(20*time.Millisecond)),
					},
				},
				{Namespace: "ns1", Name: "pod2"}: {
					Containers: map[string]storage.MetricsPoint{
						"container1": metricPoint(700, 800, scrapeTime.Add(30*time.Millisecond)),
					},
				},
				{Namespace: "ns2", Name: "pod1"}: {
					Containers: map[string]storage.MetricsPoint{
						"container1": metricPoint(900, 1000, scrapeTime.Add(40*time.Millisecond)),
					},
				},
				{Namespace: "ns3", Name: "pod1"}: {
					Containers: map[string]storage.MetricsPoint{
						"container1": metricPoint(1100, 1200, scrapeTime.Add(50*time.Millisecond)),
					},
				},
			},
		}
		client = fakeKubeletClient{
			delay: map[*corev1.Node]time.Duration{},
			metrics: map[*corev1.Node]*storage.MetricsBatch{
				node1: mb,
				node2: {Nodes: map[string]storage.MetricsPoint{node2.Name: metricPoint(100, 200, scrapeTime)}},
				node3: {Nodes: map[string]storage.MetricsPoint{node3.Name: metricPoint(100, 200, scrapeTime)}},
				node4: {Nodes: map[string]storage.MetricsPoint{node4.Name: metricPoint(100, 200, scrapeTime)}},
			},
		}
	})

	Context("when all nodes return in time", func() {
		It("should return the results of all nodes and pods", func() {
			By("setting up client to take 1 second to complete")
			client.defaultDelay = 1 * time.Second

			By("running the scraper with a context timeout of 3*seconds")

			manageNodeScrape := NewScraper(&client, 3*time.Second, metricResolution, store)
			start := time.Now()
			res1, err := manageNodeScrape.ScrapeData(node1)
			Expect(err).NotTo(HaveOccurred())
			By("ensuring that the full time took at most 3 seconds")
			Expect(time.Since(start)).To(BeNumerically("<=", 3*time.Second))

			res2, err := manageNodeScrape.ScrapeData(node2)
			Expect(err).NotTo(HaveOccurred())
			res3, err := manageNodeScrape.ScrapeData(node3)
			Expect(err).NotTo(HaveOccurred())
			res4, err := manageNodeScrape.ScrapeData(node4)
			Expect(err).NotTo(HaveOccurred())
			By("ensuring that all the nodeLister are listed")
			Expect(nodeNames(res1, res2, res3, res4)).To(ConsistOf([]string{"node1", "node-no-host", "node3", "node4"}))
			By("ensuring that all pods are present")
			Expect(podNames(res1, res2, res3, res4)).To(ConsistOf([]string{"ns1/pod1", "ns1/pod2", "ns2/pod1", "ns3/pod1"}))
		})
	})

	Context("when some clients take too long", func() {
		It("should pass the scrape timeout to the source context, so that sources can time out", func() {
			By("setting up one source to take 4 seconds, and another to take 2")
			client.delay[node1] = 4 * time.Second
			client.defaultDelay = 2 * time.Second

			By("running the source scraper with a scrape timeout of 3 seconds")
			manageNodeScrape := NewScraper(&client, 3*time.Second, metricResolution, store)
			start := time.Now()
			res1, err := manageNodeScrape.ScrapeData(node1)
			Expect(err).To(HaveOccurred())
			By("ensuring that scraping took around 3 seconds")
			Expect(time.Since(start)).To(BeNumerically("~", 3*time.Second, timeDrift))
			res2, err := manageNodeScrape.ScrapeData(node2)
			Expect(err).NotTo(HaveOccurred())
			res3, err := manageNodeScrape.ScrapeData(node3)
			Expect(err).NotTo(HaveOccurred())
			res4, err := manageNodeScrape.ScrapeData(node4)
			Expect(err).NotTo(HaveOccurred())
			By("ensuring that an error and partial results (data from source 2) were returned")
			Expect(nodeNames(res1, res2, res3, res4)).To(ConsistOf([]string{"node-no-host", "node3", "node4"}))
			Expect(podNames(res1, res2, res3, res4)).To(BeEmpty())
		})
	})

	It("should properly calculates metrics", func() {
		requestDuration.Create(nil)
		requestTotal.Create(nil)
		lastRequestTime.Create(nil)
		requestDuration.Reset()
		requestTotal.Reset()
		lastRequestTime.Reset()

		client.defaultDelay = 1 * time.Second
		myClock = mockClock{
			now:   time.Time{},
			later: time.Time{}.Add(time.Second),
		}
		manageNodeScrape := NewScraper(&client, 3*time.Second, metricResolution, store)
		_, err := manageNodeScrape.ScrapeData(node1)
		Expect(err).NotTo(HaveOccurred())
		err = testutil.CollectAndCompare(requestDuration, strings.NewReader(`
		# HELP metrics_server_kubelet_request_duration_seconds [ALPHA] Duration of requests to Kubelet API in seconds
		# TYPE metrics_server_kubelet_request_duration_seconds histogram
		metrics_server_kubelet_request_duration_seconds_bucket{node="node1",le="0.005"} 0
		metrics_server_kubelet_request_duration_seconds_bucket{node="node1",le="0.01"} 0
		metrics_server_kubelet_request_duration_seconds_bucket{node="node1",le="0.025"} 0
		metrics_server_kubelet_request_duration_seconds_bucket{node="node1",le="0.05"} 0
		metrics_server_kubelet_request_duration_seconds_bucket{node="node1",le="0.1"} 0
		metrics_server_kubelet_request_duration_seconds_bucket{node="node1",le="0.25"} 0
		metrics_server_kubelet_request_duration_seconds_bucket{node="node1",le="0.5"} 0
		metrics_server_kubelet_request_duration_seconds_bucket{node="node1",le="1"} 1
		metrics_server_kubelet_request_duration_seconds_bucket{node="node1",le="2.5"} 1
		metrics_server_kubelet_request_duration_seconds_bucket{node="node1",le="5"} 1
		metrics_server_kubelet_request_duration_seconds_bucket{node="node1",le="10"} 1
		metrics_server_kubelet_request_duration_seconds_bucket{node="node1",le="+Inf"} 1
		metrics_server_kubelet_request_duration_seconds_sum{node="node1"} 1
		metrics_server_kubelet_request_duration_seconds_count{node="node1"} 1
		`), "metrics_server_kubelet_request_duration_seconds")
		Expect(err).NotTo(HaveOccurred())

		err = testutil.CollectAndCompare(requestTotal, strings.NewReader(`
		# HELP metrics_server_kubelet_request_total [ALPHA] Number of requests sent to Kubelet API
		# TYPE metrics_server_kubelet_request_total counter
		metrics_server_kubelet_request_total{success="true"} 1
		`), "metrics_server_kubelet_request_total")
		Expect(err).NotTo(HaveOccurred())

		err = testutil.CollectAndCompare(lastRequestTime, strings.NewReader(`
		# HELP metrics_server_kubelet_last_request_time_seconds [ALPHA] Time of last request performed to Kubelet API since unix epoch in seconds
		# TYPE metrics_server_kubelet_last_request_time_seconds gauge
		metrics_server_kubelet_last_request_time_seconds{node="node1"} -6.21355968e+10
		`), "metrics_server_kubelet_last_request_time_seconds")
		Expect(err).NotTo(HaveOccurred())
	})

	It("should continue on error fetching node information for a particular node", func() {
		By("deleting node")

		delete(client.metrics, node1)
		manageNodeScrape := NewScraper(&client, 3*time.Second, metricResolution, store)
		By("running the scraper")
		res1, err := manageNodeScrape.ScrapeData(node1)
		Expect(err).To(HaveOccurred())
		res2, err := manageNodeScrape.ScrapeData(node2)
		Expect(err).NotTo(HaveOccurred())
		res3, err := manageNodeScrape.ScrapeData(node3)
		Expect(err).NotTo(HaveOccurred())
		res4, err := manageNodeScrape.ScrapeData(node4)
		Expect(err).NotTo(HaveOccurred())
		By("ensuring that all other node were scraped")
		Expect(nodeNames(res1, res2, res3, res4)).To(ConsistOf([]string{"node4", "node-no-host", "node3"}))
	})
})

func metricPoint(cpu, memory uint64, time time.Time) storage.MetricsPoint {
	return storage.MetricsPoint{
		Timestamp:         time,
		CumulativeCpuUsed: cpu,
		MemoryUsage:       memory,
	}
}

type fakeKubeletClient struct {
	delay        map[*corev1.Node]time.Duration
	metrics      map[*corev1.Node]*storage.MetricsBatch
	defaultDelay time.Duration
}

var _ client.KubeletMetricsGetter = (*fakeKubeletClient)(nil)

func (c *fakeKubeletClient) GetMetrics(ctx context.Context, node *corev1.Node) (*storage.MetricsBatch, error) {
	delay, ok := c.delay[node]
	if !ok {
		delay = c.defaultDelay
	}
	metrics, ok := c.metrics[node]
	if !ok {
		return nil, fmt.Errorf("Unknown node %q", node.Name)
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("timed out")
	case <-time.After(delay):
	}
	return metrics, nil
}

func makeNode(name, hostName, addr string, ready bool) *corev1.Node {
	res := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{},
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady},
			},
		},
	}
	if hostName != "" {
		res.Status.Addresses = append(res.Status.Addresses, corev1.NodeAddress{Type: corev1.NodeHostName, Address: hostName})
	}
	if addr != "" {
		res.Status.Addresses = append(res.Status.Addresses, corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: addr})
	}
	if ready {
		res.Status.Conditions[0].Status = corev1.ConditionTrue
	} else {
		res.Status.Conditions[0].Status = corev1.ConditionFalse
	}
	return res
}

func nodeNames(batchs ...*storage.MetricsBatch) []string {
	var names []string
	for _, batch := range batchs {
		if batch == nil {
			continue
		}
		for node := range batch.Nodes {
			names = append(names, node)
		}
	}
	return names
}

func podNames(batchs ...*storage.MetricsBatch) []string {
	var names []string
	for _, batch := range batchs {
		if batch == nil {
			continue
		}
		for pod := range batch.Pods {
			names = append(names, pod.Namespace+"/"+pod.Name)
		}
	}
	return names
}

type mockClock struct {
	now   time.Time
	later time.Time
}

func (c mockClock) Now() time.Time                  { return c.now }
func (c mockClock) Since(d time.Time) time.Duration { return c.later.Sub(d) }
