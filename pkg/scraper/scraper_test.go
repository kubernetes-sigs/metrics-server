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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/component-base/metrics/testutil"

	"sigs.k8s.io/metrics-server/pkg/storage"
)

const timeDrift = 50 * time.Millisecond

func TestScraper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scraper Suite")
}

var _ = Describe("Scraper", func() {
	var (
		scrapeTime = time.Now()
		nodeLister fakeNodeLister
		client     fakeKubeletClient
		node1      = makeNode("node1", "node1.somedomain", "10.0.1.2", true)
		node2      = makeNode("node-no-host", "", "10.0.1.3", true)
		node3      = makeNode("node3", "node3.somedomain", "10.0.1.4", false)
		node4      = makeNode("node4", "node4.somedomain", "10.0.1.5", true)
	)
	BeforeEach(func() {
		mb := &storage.MetricsBatch{
			Nodes: []storage.NodeMetricsPoint{nodeMetrics(node1, 100, 200, scrapeTime)},
			Pods: []storage.PodMetricsPoint{
				{Namespace: "ns1", Name: "pod1", Containers: []storage.ContainerMetricsPoint{
					{Name: "container1", MetricsPoint: metricPoint(300, 400, scrapeTime.Add(10*time.Millisecond))},
					{Name: "container2", MetricsPoint: metricPoint(500, 600, scrapeTime.Add(20*time.Millisecond))},
				}},
				{Namespace: "ns1", Name: "pod2", Containers: []storage.ContainerMetricsPoint{
					{Name: "container1", MetricsPoint: metricPoint(700, 800, scrapeTime.Add(30*time.Millisecond))},
				}},
				{Namespace: "ns2", Name: "pod1", Containers: []storage.ContainerMetricsPoint{
					{Name: "container1", MetricsPoint: metricPoint(900, 1000, scrapeTime.Add(40*time.Millisecond))},
				}},
				{Namespace: "ns3", Name: "pod1", Containers: []storage.ContainerMetricsPoint{
					{Name: "container1", MetricsPoint: metricPoint(1100, 1200, scrapeTime.Add(50*time.Millisecond))},
				}},
			},
		}
		nodeLister = fakeNodeLister{nodes: []*corev1.Node{node1, node2, node3, node4}}
		client = fakeKubeletClient{
			delay: map[*corev1.Node]time.Duration{},
			metrics: map[*corev1.Node]*storage.MetricsBatch{
				node1: mb,
				node2: {Nodes: []storage.NodeMetricsPoint{nodeMetrics(node2, 100, 200, scrapeTime)}},
				node3: {Nodes: []storage.NodeMetricsPoint{nodeMetrics(node3, 100, 200, scrapeTime)}},
				node4: {Nodes: []storage.NodeMetricsPoint{nodeMetrics(node4, 100, 200, scrapeTime)}},
			},
		}
	})

	Context("when all nodes return in time", func() {
		It("should return the results of all nodes and pods", func() {
			By("setting up client to take 1 second to complete")
			client.defaultDelay = 1 * time.Second

			By("running the scraper with a context timeout of 3*seconds")
			start := time.Now()
			scraper := NewScraper(&nodeLister, &client, 3*time.Second)
			timeoutCtx, doneWithWork := context.WithTimeout(context.Background(), 4*time.Second)
			dataBatch := scraper.Scrape(timeoutCtx)
			doneWithWork()

			By("ensuring that the full time took at most 3 seconds")
			Expect(time.Since(start)).To(BeNumerically("<=", 3*time.Second))

			By("ensuring that all the nodeLister are listed")
			Expect(nodeNames(dataBatch.Nodes)).To(ConsistOf([]string{"node1", "node-no-host", "node3", "node4"}))
			By("ensuring that all pods are present")
			Expect(podNames(dataBatch.Pods)).To(ConsistOf([]string{"ns1/pod1", "ns1/pod2", "ns2/pod1", "ns3/pod1"}))
		})
	})

	Context("when some clients take too long", func() {
		It("should pass the scrape timeout to the source context, so that sources can time out", func() {
			By("setting up one source to take 4 seconds, and another to take 2")
			client.delay[node1] = 4 * time.Second
			client.defaultDelay = 2 * time.Second

			By("running the source scraper with a scrape timeout of 3 seconds")
			start := time.Now()
			scraper := NewScraper(&nodeLister, &client, 3*time.Second)
			dataBatch := scraper.Scrape(context.Background())

			By("ensuring that scraping took around 3 seconds")
			Expect(time.Since(start)).To(BeNumerically("~", 3*time.Second, timeDrift))

			By("ensuring that an error and partial results (data from source 2) were returned")
			Expect(nodeNames(dataBatch.Nodes)).To(ConsistOf([]string{"node-no-host", "node3", "node4"}))
			Expect(podNames(dataBatch.Pods)).To(BeEmpty())
		})

		It("should respect the parent context's general timeout, even with a longer scrape timeout", func() {
			By("setting up some sources with 4 second delays")
			client.defaultDelay = 4 * time.Second

			By("running the source scraper with a scrape timeout of 5 seconds, but a context timeout of 1 second")
			start := time.Now()
			scraper := NewScraper(&nodeLister, &client, 5*time.Second)
			timeoutCtx, doneWithWork := context.WithTimeout(context.Background(), 1*time.Second)
			dataBatch := scraper.Scrape(timeoutCtx)
			doneWithWork()

			By("ensuring that it times out after 1 second with errors and no data")
			Expect(time.Since(start)).To(BeNumerically("~", 1*time.Second, timeDrift))
			Expect(dataBatch.Nodes).To(BeEmpty())
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
		nodes := fakeNodeLister{nodes: []*corev1.Node{node1}}

		scraper := NewScraper(&nodes, &client, 3*time.Second)
		scraper.Scrape(context.Background())

		err := testutil.CollectAndCompare(requestDuration, strings.NewReader(`
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
		nodeLister.nodes[0].Status.Addresses = nil
		delete(client.metrics, node1)
		scraper := NewScraper(&nodeLister, &client, 5*time.Second)

		By("running the scraper")
		dataBatch := scraper.Scrape(context.Background())

		By("ensuring that all other node were scraped")
		Expect(nodeNames(dataBatch.Nodes)).To(ConsistOf([]string{"node4", "node-no-host", "node3"}))
	})
	It("should gracefully handle list errors", func() {
		By("setting a fake error from the lister")
		nodeLister.listErr = fmt.Errorf("something went wrong, expectedly")
		scraper := NewScraper(&nodeLister, &client, 5*time.Second)

		By("running the scraper")
		scraper.Scrape(context.Background())
	})
})

func nodeMetrics(node *corev1.Node, cpu, memory uint64, scrapeTime time.Time) storage.NodeMetricsPoint {
	return storage.NodeMetricsPoint{
		Name: node.Name,
		MetricsPoint: storage.MetricsPoint{
			CumulativeCpuUsed: cpu,
			MemoryUsage:       memory,
			Timestamp:         scrapeTime,
		},
	}
}

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

var _ client.KubeletMetricsInterface = (*fakeKubeletClient)(nil)

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

func (l *fakeNodeLister) Get(name string) (*corev1.Node, error) {
	for _, node := range l.nodes {
		if node.Name == name {
			return node, nil
		}
	}
	return nil, fmt.Errorf("no such node %q", name)
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

func nodeNames(nodes []storage.NodeMetricsPoint) []string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.Name)
	}
	return names
}

func podNames(pods []storage.PodMetricsPoint) []string {
	names := make([]string, 0, len(pods))
	for _, pod := range pods {
		names = append(names, pod.Namespace+"/"+pod.Name)
	}
	return names
}

type mockClock struct {
	now   time.Time
	later time.Time
}

func (c mockClock) Now() time.Time                  { return c.now }
func (c mockClock) Since(d time.Time) time.Duration { return c.later.Sub(d) }
