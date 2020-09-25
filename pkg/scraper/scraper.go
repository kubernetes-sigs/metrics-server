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
			Help:      "Duration of requests to kubelet API in seconds",
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

func init() {
	legacyregistry.MustRegister(requestDuration)
	legacyregistry.MustRegister(lastRequestTime)
	legacyregistry.MustRegister(requestTotal)
}

func NewScraper(nodeLister v1listers.NodeLister, client KubeletInterface, scrapeTimeout time.Duration) *scraper {
	return &scraper{
		nodeLister:    nodeLister,
		kubeletClient: client,
		scrapeTimeout: scrapeTimeout,
	}
}

type scraper struct {
	nodeLister    v1listers.NodeLister
	kubeletClient KubeletInterface
	scrapeTimeout time.Duration
}

var _ Scraper = (*scraper)(nil)

// NodeInfo contains the information needed to identify and connect to a particular node
// (node name and preferred address).
type NodeInfo struct {
	Name           string
	ConnectAddress string
}

func (c *scraper) Scrape(baseCtx context.Context) (*storage.MetricsBatch, error) {
	nodes, err := c.nodeLister.List(labels.Everything())
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
		go func(node *corev1.Node) {
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

func (c *scraper) collectNode(ctx context.Context, node *corev1.Node) (*storage.MetricsBatch, error) {
	startTime := myClock.Now()
	defer func() {
		requestDuration.WithLabelValues(node.Name).Observe(float64(myClock.Since(startTime)) / float64(time.Second))
		lastRequestTime.WithLabelValues(node.Name).Set(float64(myClock.Now().Unix()))
	}()
	summary, err := c.kubeletClient.GetSummary(ctx, node)

	if err != nil {
		requestTotal.WithLabelValues("false").Inc()
		return nil, fmt.Errorf("unable to fetch metrics from node %s: %v", node.Name, err)
	}
	requestTotal.WithLabelValues("true").Inc()
	return decodeBatch(summary), nil
}

type clock interface {
	Now() time.Time
	Since(time.Time) time.Duration
}

type realClock struct{}

func (realClock) Now() time.Time                  { return time.Now() }
func (realClock) Since(d time.Time) time.Duration { return time.Since(d) }

var myClock clock = &realClock{}
