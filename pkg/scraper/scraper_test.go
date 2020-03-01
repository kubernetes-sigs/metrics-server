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

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/component-base/metrics/testutil"
	"sigs.k8s.io/metrics-server/pkg/storage"
	"sigs.k8s.io/metrics-server/pkg/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1listers "k8s.io/client-go/listers/core/v1"

	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

const timeDrift = 50 * time.Millisecond

func TestScraper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scraper Suite")
}

func nodeStats(host string, cpu, memory int, scrapeTime time.Time) stats.NodeStats {
	return stats.NodeStats{
		NodeName: host,
		CPU:      cpuStats(100, scrapeTime.Add(100*time.Millisecond)),
		Memory:   memStats(200, scrapeTime.Add(200*time.Millisecond)),
	}
}

var _ = Describe("Scraper", func() {
	var (
		scrapeTime = time.Now()
		nodeLister fakeNodeLister
		resolver   utils.NodeAddressResolver
		client     fakeKubeletClient
	)
	BeforeEach(func() {
		summary := &stats.Summary{
			Node: nodeStats("node1", 100, 200, scrapeTime),
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
		}
		resolver = utils.NewPriorityNodeAddressResolver(utils.DefaultAddressTypePriority)
		nodeLister = fakeNodeLister{nodes: []*corev1.Node{
			makeNode("node1", "node1.somedomain", "10.0.1.2", true),
			makeNode("node-no-host", "", "10.0.1.3", true),
			makeNode("node3", "node3.somedomain", "10.0.1.4", false),
			makeNode("node4", "node4.somedomain", "10.0.1.5", true),
		}}
		client = fakeKubeletClient{
			delay: map[string]time.Duration{},
			metrics: map[string]*stats.Summary{
				"node1.somedomain": summary,
				"10.0.1.3":         {Node: nodeStats("node-no-host", 100, 200, scrapeTime)},
				"node3.somedomain": {Node: nodeStats("node3", 100, 200, scrapeTime)},
				"node4.somedomain": {Node: nodeStats("node4", 100, 200, scrapeTime)},
			},
		}
	})

	Context("when all nodes return in time", func() {
		It("should return the results of all nodes and pods", func() {
			By("setting up client to take 1 second to complete")
			client.defaultDelay = 1 * time.Second

			By("running the scraper with a context timeout of 3*seconds")
			start := time.Now()
			scraper := NewScraper(&nodeLister, &client, resolver, 3*time.Second)
			timeoutCtx, doneWithWork := context.WithTimeout(context.Background(), 4*time.Second)
			dataBatch, errs := scraper.Scrape(timeoutCtx)
			doneWithWork()
			Expect(errs).NotTo(HaveOccurred())

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
			client.delay["node1.somedomain"] = 4 * time.Second
			client.defaultDelay = 2 * time.Second

			By("running the source scraper with a scrape timeout of 3 seconds")
			start := time.Now()
			scraper := NewScraper(&nodeLister, &client, resolver, 3*time.Second)
			dataBatch, errs := scraper.Scrape(context.Background())
			Expect(errs).To(HaveOccurred())

			By("ensuring that scraping took around 3 seconds")
			Expect(time.Since(start)).To(BeNumerically("~", 3*time.Second, timeDrift))

			By("ensuring that an error and partial results (data from source 2) were returned")
			Expect(errs).To(HaveOccurred())
			Expect(nodeNames(dataBatch.Nodes)).To(ConsistOf([]string{"node-no-host", "node3", "node4"}))
			Expect(podNames(dataBatch.Pods)).To(BeEmpty())
		})

		It("should respect the parent context's general timeout, even with a longer scrape timeout", func() {
			By("setting up some sources with 4 second delays")
			client.defaultDelay = 4 * time.Second

			By("running the source scraper with a scrape timeout of 5 seconds, but a context timeout of 1 second")
			start := time.Now()
			scraper := NewScraper(&nodeLister, &client, resolver, 5*time.Second)
			timeoutCtx, doneWithWork := context.WithTimeout(context.Background(), 1*time.Second)
			dataBatch, errs := scraper.Scrape(timeoutCtx)
			doneWithWork()
			Expect(errs).To(HaveOccurred())

			By("ensuring that it times out after 1 second with errors and no data")
			Expect(time.Since(start)).To(BeNumerically("~", 1*time.Second, timeDrift))
			Expect(errs).To(HaveOccurred())
			Expect(dataBatch.Nodes).To(BeEmpty())
		})
	})

	It("should properly calculates metrics", func() {
		summaryRequestLatency.Reset()
		scrapeTotal.Reset()
		lastScrapeTimestamp.Reset()
		client.defaultDelay = 1 * time.Second
		myClock = mockClock{
			now:   time.Time{},
			later: time.Time{}.Add(time.Second),
		}
		nodes := fakeNodeLister{nodes: []*corev1.Node{
			makeNode("node1", "node1.somedomain", "10.0.1.2", true),
		}}

		scraper := NewScraper(&nodes, &client, resolver, 3*time.Second)
		_, errs := scraper.Scrape(context.Background())
		Expect(errs).NotTo(HaveOccurred())

		err := testutil.CollectAndCompare(summaryRequestLatency, strings.NewReader(`
		# HELP metrics_server_kubelet_summary_request_duration_seconds [ALPHA] The Kubelet summary request latencies in seconds.
		# TYPE metrics_server_kubelet_summary_request_duration_seconds histogram
		metrics_server_kubelet_summary_request_duration_seconds_bucket{node="node1",le="0.005"} 0
		metrics_server_kubelet_summary_request_duration_seconds_bucket{node="node1",le="0.01"} 0
		metrics_server_kubelet_summary_request_duration_seconds_bucket{node="node1",le="0.025"} 0
		metrics_server_kubelet_summary_request_duration_seconds_bucket{node="node1",le="0.05"} 0
		metrics_server_kubelet_summary_request_duration_seconds_bucket{node="node1",le="0.1"} 0
		metrics_server_kubelet_summary_request_duration_seconds_bucket{node="node1",le="0.25"} 0
		metrics_server_kubelet_summary_request_duration_seconds_bucket{node="node1",le="0.5"} 0
		metrics_server_kubelet_summary_request_duration_seconds_bucket{node="node1",le="1"} 1
		metrics_server_kubelet_summary_request_duration_seconds_bucket{node="node1",le="2.5"} 1
		metrics_server_kubelet_summary_request_duration_seconds_bucket{node="node1",le="5"} 1
		metrics_server_kubelet_summary_request_duration_seconds_bucket{node="node1",le="10"} 1
		metrics_server_kubelet_summary_request_duration_seconds_bucket{node="node1",le="+Inf"} 1
		metrics_server_kubelet_summary_request_duration_seconds_sum{node="node1"} 1
		metrics_server_kubelet_summary_request_duration_seconds_count{node="node1"} 1
		`), "metrics_server_kubelet_summary_request_duration_seconds")
		Expect(err).NotTo(HaveOccurred())

		err = testutil.CollectAndCompare(scrapeTotal, strings.NewReader(`
		# HELP metrics_server_kubelet_summary_scrapes_total [ALPHA] Total number of attempted Summary API scrapes done by Metrics Server
		# TYPE metrics_server_kubelet_summary_scrapes_total counter
		metrics_server_kubelet_summary_scrapes_total{success="true"} 1
		`), "metrics_server_kubelet_summary_scrapes_total")
		Expect(err).NotTo(HaveOccurred())

		err = testutil.CollectAndCompare(lastScrapeTimestamp, strings.NewReader(`
		# HELP metrics_server_scraper_last_time_seconds [ALPHA] Last time metrics-server performed a scrape since unix epoch in seconds.
		# TYPE metrics_server_scraper_last_time_seconds gauge
		metrics_server_scraper_last_time_seconds{source="node1"} -6.21355968e+10
		`), "metrics_server_scraper_last_time_seconds")
		Expect(err).NotTo(HaveOccurred())
	})

	It("should continue on error fetching node information for a particular node", func() {
		By("deleting the IP of a node")
		nodeLister.nodes[0].Status.Addresses = nil
		scraper := NewScraper(&nodeLister, &client, resolver, 5*time.Second)

		By("listing the nodes")
		nodes, err := scraper.GetNodes()
		Expect(err).To(HaveOccurred())

		By("verifying that a source is present for each node")
		Expect(nodes).To(HaveLen(3))
	})
	It("should gracefully handle list errors", func() {
		By("setting a fake error from the lister")
		nodeLister.listErr = fmt.Errorf("something went wrong, expectedly")
		scraper := NewScraper(&nodeLister, &client, resolver, 5*time.Second)

		By("listing the sources")
		_, err := scraper.GetNodes()
		Expect(err).To(HaveOccurred())
	})

	It("should prefer addresses according to the order of the types first", func() {
		By("setting the first node to have multiple addresses and setting all nodeLister to ready")
		nodeLister.nodes[0].Status.Addresses = []corev1.NodeAddress{
			{Type: utils.DefaultAddressTypePriority[3], Address: "skip-val1"},
			{Type: utils.DefaultAddressTypePriority[2], Address: "skip-val2"},
			{Type: utils.DefaultAddressTypePriority[1], Address: "correct-val"},
		}
		scraper := NewScraper(&nodeLister, &client, resolver, 5*time.Second)
		By("listing all sources")
		nodes, err := scraper.GetNodes()
		Expect(err).NotTo(HaveOccurred())

		By("making sure that the first source scrapes from the correct location")
		Expect(nodes[0].ConnectAddress).To(Equal("correct-val"))
	})

	It("should prefer the first address that matches within a given type", func() {
		By("setting the first node to have multiple addresses and setting all nodeLister to ready")
		nodeLister.nodes[0].Status.Addresses = []corev1.NodeAddress{
			{Type: utils.DefaultAddressTypePriority[1], Address: "skip-val1"},
			{Type: utils.DefaultAddressTypePriority[0], Address: "correct-val"},
			{Type: utils.DefaultAddressTypePriority[1], Address: "skip-val2"},
			{Type: utils.DefaultAddressTypePriority[0], Address: "second-val"},
		}
		scraper := NewScraper(&nodeLister, &client, resolver, 5*time.Second)

		By("listing all sources")
		nodes, err := scraper.GetNodes()
		Expect(err).NotTo(HaveOccurred())

		By("making sure that the first source scrapes from the correct location")
		Expect(nodes[0].ConnectAddress).To(Equal("correct-val"))
	})

	It("should return an error if no preferred addresses are found", func() {
		By("wiping out the addresses of one of the nodeLister and setting all nodeLister to ready")
		nodeLister.nodes[0].Status.Addresses = nil
		scraper := NewScraper(&nodeLister, &client, resolver, 5*time.Second)

		By("asking for scraper for all nodeLister")
		_, err := scraper.GetNodes()
		Expect(err).To(HaveOccurred())
	})
})

type fakeKubeletClient struct {
	delay        map[string]time.Duration
	metrics      map[string]*stats.Summary
	defaultDelay time.Duration
}

func (c *fakeKubeletClient) GetSummary(ctx context.Context, host string) (*stats.Summary, error) {
	delay, ok := c.delay[host]
	if !ok {
		delay = c.defaultDelay
	}
	metrics, ok := c.metrics[host]
	if !ok {
		return nil, fmt.Errorf("Unknown host %q", host)
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

func (l *fakeNodeLister) ListWithPredicate(_ v1listers.NodeConditionPredicate) ([]*corev1.Node, error) {
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
