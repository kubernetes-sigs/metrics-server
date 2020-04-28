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
	"math/rand"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
	"k8s.io/klog"

	"sigs.k8s.io/metrics-server/pkg/storage"
	"sigs.k8s.io/metrics-server/pkg/utils"
)

const (
	maxDelayMs       = 4 * 1000
	delayPerSourceMs = 8
)

var (
	summaryRequestLatency = metrics.NewHistogramVec(
		&metrics.HistogramOpts{
			Namespace: "metrics_server",
			Subsystem: "kubelet_summary",
			Name:      "request_duration_seconds",
			Help:      "The Kubelet summary request latencies in seconds.",
			// TODO(directxman12): it would be nice to calculate these buckets off of scrape duration,
			// like we do elsewhere, but we're not passed the scrape duration at this level.
			Buckets: metrics.DefBuckets,
		},
		[]string{"node"},
	)
	scrapeTotal = metrics.NewCounterVec(
		&metrics.CounterOpts{
			Namespace: "metrics_server",
			Subsystem: "kubelet_summary",
			Name:      "scrapes_total",
			Help:      "Total number of attempted Summary API scrapes done by Metrics Server",
		},
		[]string{"success"},
	)
	lastScrapeTimestamp = metrics.NewGaugeVec(
		&metrics.GaugeOpts{
			Namespace: "metrics_server",
			Subsystem: "scraper",
			Name:      "last_time_seconds",
			Help:      "Last time metrics-server performed a scrape since unix epoch in seconds.",
		},
		[]string{"source"},
	)

	// initialized below to an actual value by a call to RegisterScraperDuration
	// (acts as a no-op by default), but we can't just register it in the constructor,
	// since it could be called multiple times during setup.
	scraperDuration *metrics.HistogramVec = metrics.NewHistogramVec(&metrics.HistogramOpts{}, []string{"source"})
)

func init() {
	legacyregistry.MustRegister(summaryRequestLatency)
	legacyregistry.MustRegister(lastScrapeTimestamp)
	legacyregistry.MustRegister(scrapeTotal)
}

// RegisterScraperMetrics creates and registers a histogram metric for
// scrape duration, suitable for use in the source manager.
func RegisterScraperMetrics(scrapeTimeout time.Duration) {
	scraperDuration = metrics.NewHistogramVec(
		&metrics.HistogramOpts{
			Namespace: "metrics_server",
			Subsystem: "scraper",
			Name:      "duration_seconds",
			Help:      "Time spent scraping sources in seconds.",
			Buckets:   utils.BucketsForScrapeDuration(scrapeTimeout),
		},
		[]string{"source"},
	)
	legacyregistry.MustRegister(scraperDuration)
}

func NewScraper(nodeLister v1listers.NodeLister, client KubeletInterface, addrResolver utils.NodeAddressResolver, scrapeTimeout time.Duration) *Scraper {
	return &Scraper{
		nodeLister:    nodeLister,
		kubeletClient: client,
		addrResolver:  addrResolver,
		scrapeTimeout: scrapeTimeout,
	}
}

type Scraper struct {
	nodeLister    v1listers.NodeLister
	kubeletClient KubeletInterface
	addrResolver  utils.NodeAddressResolver
	scrapeTimeout time.Duration
}

// NodeInfo contains the information needed to identify and connect to a particular node
// (node name and preferred address).
type NodeInfo struct {
	Name           string
	ConnectAddress string
}

func (c *Scraper) Scrape(baseCtx context.Context) (*storage.MetricsBatch, error) {
	nodes, err := c.GetNodes()
	var errs []error
	if err != nil {
		// save the error, and continue on in case of partial results
		errs = append(errs, err)
	}
	klog.V(1).Infof("Scraping metrics from %v nodes", len(nodes))

	responseChannel := make(chan *storage.MetricsBatch, len(nodes))
	errChannel := make(chan error, len(nodes))
	defer close(responseChannel)
	defer close(errChannel)

	startTime := myClock.Now()

	// TODO(serathius): re-evaluate this code -- do we really need to stagger fetches like this?
	delayMs := delayPerSourceMs * len(nodes)
	if delayMs > maxDelayMs {
		delayMs = maxDelayMs
	}

	for _, node := range nodes {
		go func(node NodeInfo) {
			// Prevents network congestion.
			sleepDuration := time.Duration(rand.Intn(delayMs)) * time.Millisecond
			time.Sleep(sleepDuration)
			// make the timeout a bit shorter to account for staggering, so we still preserve
			// the overall timeout
			ctx, cancelTimeout := context.WithTimeout(baseCtx, c.scrapeTimeout-sleepDuration)
			defer cancelTimeout()

			klog.V(2).Infof("Querying source: %s", node)
			metrics, err := c.collectNode(ctx, node)
			if err != nil {
				err = fmt.Errorf("unable to fully scrape metrics from node %s: %v", node.Name, err)
			}
			responseChannel <- metrics
			errChannel <- err
		}(node)
	}

	res := &storage.MetricsBatch{}

	for range nodes {
		err := <-errChannel
		srcBatch := <-responseChannel
		if err != nil {
			errs = append(errs, err)
			// NB: partial node results are still worth saving, so
			// don't skip storing results if we got an error
		}
		if srcBatch == nil {
			continue
		}

		res.Nodes = append(res.Nodes, srcBatch.Nodes...)
		res.Pods = append(res.Pods, srcBatch.Pods...)
	}

	klog.V(1).Infof("ScrapeMetrics: time: %s, nodes: %v, pods: %v", myClock.Since(startTime), len(res.Nodes), len(res.Pods))
	return res, utilerrors.NewAggregate(errs)
}

func (c *Scraper) collectNode(ctx context.Context, node NodeInfo) (*storage.MetricsBatch, error) {
	startTime := myClock.Now()
	defer func() {
		scraperDuration.WithLabelValues(node.Name).Observe(float64(myClock.Since(startTime)) / float64(time.Second))
		summaryRequestLatency.WithLabelValues(node.Name).Observe(float64(myClock.Since(startTime)) / float64(time.Second))
		lastScrapeTimestamp.WithLabelValues(node.Name).Set(float64(myClock.Now().Unix()))
	}()
	summary, err := c.kubeletClient.GetSummary(ctx, node.ConnectAddress)

	if err != nil {
		scrapeTotal.WithLabelValues("false").Inc()
		return nil, fmt.Errorf("unable to fetch metrics from Kubelet %s (%s): %v", node.Name, node.ConnectAddress, err)
	}
	scrapeTotal.WithLabelValues("true").Inc()
	return decodeBatch(summary), nil
}

func (c *Scraper) GetNodes() ([]NodeInfo, error) {
	nodes, err := c.nodeLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("unable to list nodes: %v", err)
	}

	var errs []error
	result := make([]NodeInfo, 0, len(nodes))
	for _, node := range nodes {
		info, err := c.getNodeInfo(node)
		if err != nil {
			errs = append(errs, fmt.Errorf("unable to extract connection information for node %q: %v", node.Name, err))
			continue
		}
		result = append(result, info)
	}
	return result, utilerrors.NewAggregate(errs)
}

func (c *Scraper) getNodeInfo(node *corev1.Node) (NodeInfo, error) {
	addr, err := c.addrResolver.NodeAddress(node)
	if err != nil {
		return NodeInfo{}, err
	}
	info := NodeInfo{
		Name:           node.Name,
		ConnectAddress: addr,
	}

	return info, nil
}

type clock interface {
	Now() time.Time
	Since(time.Time) time.Duration
}

type realClock struct{}

func (realClock) Now() time.Time                  { return time.Now() }
func (realClock) Since(d time.Time) time.Duration { return time.Since(d) }

var myClock clock = &realClock{}
