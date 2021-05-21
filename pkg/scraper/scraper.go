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
	"math/rand"
	"time"

	"sigs.k8s.io/metrics-server/pkg/scraper/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/component-base/metrics"
	"k8s.io/klog/v2"

	"sigs.k8s.io/metrics-server/pkg/storage"
)

const (
	maxDelayMs       = 4 * 1000
	delayPerSourceMs = 8
)

var (
	requestDuration = metrics.NewHistogramVec(
		&metrics.HistogramOpts{
			Namespace: "metrics_server",
			Subsystem: "kubelet",
			Name:      "request_duration_seconds",
			Help:      "Duration of requests to Kubelet API in seconds",
			Buckets:   metrics.DefBuckets,
		},
		[]string{"node"},
	)
	requestTotal = metrics.NewCounterVec(
		&metrics.CounterOpts{
			Namespace: "metrics_server",
			Subsystem: "kubelet",
			Name:      "request_total",
			Help:      "Number of requests sent to Kubelet API",
		},
		[]string{"success"},
	)
	lastRequestTime = metrics.NewGaugeVec(
		&metrics.GaugeOpts{
			Namespace: "metrics_server",
			Subsystem: "kubelet",
			Name:      "last_request_time_seconds",
			Help:      "Time of last request performed to Kubelet API since unix epoch in seconds",
		},
		[]string{"node"},
	)
)

// RegisterScraperMetrics registers rate, errors, and duration metrics on
// Kubelet API scrapes.
func RegisterScraperMetrics(registrationFunc func(metrics.Registerable) error) error {
	for _, metric := range []metrics.Registerable{
		requestDuration,
		requestTotal,
		lastRequestTime,
	} {
		err := registrationFunc(metric)
		if err != nil {
			return err
		}
	}
	return nil
}

func NewScraper(nodeLister v1listers.NodeLister, client client.KubeletMetricsInterface, scrapeTimeout time.Duration) *scraper {
	return &scraper{
		nodeLister:    nodeLister,
		kubeletClient: client,
		scrapeTimeout: scrapeTimeout,
	}
}

type scraper struct {
	nodeLister    v1listers.NodeLister
	kubeletClient client.KubeletMetricsInterface
	scrapeTimeout time.Duration
}

var _ Scraper = (*scraper)(nil)

// NodeInfo contains the information needed to identify and connect to a particular node
// (node name and preferred address).
type NodeInfo struct {
	Name           string
	ConnectAddress string
}

func (c *scraper) Scrape(baseCtx context.Context) *storage.MetricsBatch {
	nodes, err := c.nodeLister.List(labels.Everything())
	if err != nil {
		// report the error and continue on in case of partial results
		klog.ErrorS(err, "Failed to list nodes")
	}
	klog.V(1).InfoS("Scraping metrics from nodes", "nodeCount", len(nodes))

	responseChannel := make(chan *storage.MetricsBatch, len(nodes))
	defer close(responseChannel)

	startTime := myClock.Now()

	// TODO(serathius): re-evaluate this code -- do we really need to stagger fetches like this?
	delayMs := delayPerSourceMs * len(nodes)
	if delayMs > maxDelayMs {
		delayMs = maxDelayMs
	}

	for _, node := range nodes {
		go func(node *corev1.Node) {
			// Prevents network congestion.
			sleepDuration := time.Duration(rand.Intn(delayMs)) * time.Millisecond
			time.Sleep(sleepDuration)
			// make the timeout a bit shorter to account for staggering, so we still preserve
			// the overall timeout
			ctx, cancelTimeout := context.WithTimeout(baseCtx, c.scrapeTimeout-sleepDuration)
			defer cancelTimeout()
			klog.V(2).InfoS("Scraping node", "node", klog.KObj(node))
			m, err := c.collectNode(ctx, node)
			if err != nil {
				klog.ErrorS(err, "Failed to scrape node", "node", klog.KObj(node))
			}
			responseChannel <- m
		}(node)
	}

	res := &storage.MetricsBatch{}

	for range nodes {
		srcBatch := <-responseChannel
		if srcBatch == nil {
			continue
		}

		res.Nodes = append(res.Nodes, srcBatch.Nodes...)
		res.Pods = append(res.Pods, srcBatch.Pods...)
	}

	klog.V(1).InfoS("Scrape finished", "duration", myClock.Since(startTime), "nodeCount", len(res.Nodes), "podCount", len(res.Pods))
	return res
}

func (c *scraper) collectNode(ctx context.Context, node *corev1.Node) (*storage.MetricsBatch, error) {
	startTime := myClock.Now()
	defer func() {
		requestDuration.WithLabelValues(node.Name).Observe(float64(myClock.Since(startTime)) / float64(time.Second))
		lastRequestTime.WithLabelValues(node.Name).Set(float64(myClock.Now().Unix()))
	}()
	ms, err := c.kubeletClient.GetMetrics(ctx, node)

	if err != nil {
		requestTotal.WithLabelValues("false").Inc()
		return nil, err
	}
	requestTotal.WithLabelValues("true").Inc()
	return ms, nil
}

type clock interface {
	Now() time.Time
	Since(time.Time) time.Duration
}

type realClock struct{}

func (realClock) Now() time.Time                  { return time.Now() }
func (realClock) Since(d time.Time) time.Duration { return time.Since(d) }

var myClock clock = &realClock{}
