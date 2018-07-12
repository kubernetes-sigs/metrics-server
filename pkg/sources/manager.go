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

package sources

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

const (
	maxDelayMs       = 4 * 1000
	delayPerSourceMs = 8
)

var (
	// Last time Heapster performed a scrape since unix epoch in seconds.
	lastScrapeTimestamp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "heapster",
			Subsystem: "scraper",
			Name:      "last_time_seconds",
			Help:      "Last time Heapster performed a scrape since unix epoch in seconds.",
		},
		[]string{"source"},
	)

	// Time spent exporting scraping sources in microseconds..
	scraperDuration = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: "heapster",
			Subsystem: "scraper",
			Name:      "duration_microseconds",
			Help:      "Time spent scraping sources in microseconds.",
		},
		[]string{"source"},
	)
)

func init() {
	prometheus.MustRegister(lastScrapeTimestamp)
	prometheus.MustRegister(scraperDuration)
}

func NewSourceManager(srcProv MetricSourceProvider, scrapeTimeout time.Duration) MetricSource {
	return &sourceManager{
		srcProv:       srcProv,
		scrapeTimeout: scrapeTimeout,
	}
}

type sourceManager struct {
	srcProv       MetricSourceProvider
	scrapeTimeout time.Duration
}

func (m *sourceManager) Name() string {
	return "source_manager"
}

func (m *sourceManager) Collect(baseCtx context.Context) (*MetricsBatch, error) {
	sources, err := m.srcProv.GetMetricSources()
	var errs []error
	if err != nil {
		// save the error, and continue on in case of partial results
		errs = append(errs, err)
	}
	glog.V(1).Infof("Scraping metrics from %v sources", len(sources))

	responseChannel := make(chan *MetricsBatch, len(sources))
	errChannel := make(chan error, len(sources))
	defer close(responseChannel)
	defer close(errChannel)

	startTime := time.Now()

	// TODO(directxman12): re-evaluate this code -- do we really need to stagger fetches like this?
	delayMs := delayPerSourceMs * len(sources)
	if delayMs > maxDelayMs {
		delayMs = maxDelayMs
	}

	for _, source := range sources {
		go func(source MetricSource) {
			// Prevents network congestion.
			sleepDuration := time.Duration(rand.Intn(delayMs)) * time.Millisecond
			time.Sleep(sleepDuration)
			// make the timeout a bit shorter to account for staggering, so we still preserve
			// the overall timeout
			ctx, cancelTimeout := context.WithTimeout(baseCtx, m.scrapeTimeout-sleepDuration)
			defer cancelTimeout()

			glog.V(2).Infof("Querying source: %s", source)
			metrics, err := scrapeWithMetrics(ctx, source)
			if err != nil {
				errChannel <- fmt.Errorf("unable to fully scrape metrics from source %s: %v", source.Name(), err)
				responseChannel <- metrics
				return
			}
			responseChannel <- metrics
			errChannel <- nil
		}(source)
	}

	res := &MetricsBatch{}

	for range sources {
		err := <-errChannel
		srcBatch := <-responseChannel
		if err != nil {
			errs = append(errs, err)
			continue
		}

		res.Nodes = append(res.Nodes, srcBatch.Nodes...)
		res.Pods = append(res.Pods, srcBatch.Pods...)
	}

	glog.V(1).Infof("ScrapeMetrics: time: %s, nodes: %v, pods: %v", time.Since(startTime), len(res.Nodes), len(res.Pods))
	return res, utilerrors.NewAggregate(errs)
}

func scrapeWithMetrics(ctx context.Context, s MetricSource) (*MetricsBatch, error) {
	sourceName := s.Name()
	startTime := time.Now()
	defer lastScrapeTimestamp.
		WithLabelValues(sourceName).
		Set(float64(time.Now().Unix()))
	defer scraperDuration.
		WithLabelValues(sourceName).
		Observe(float64(time.Since(startTime)) / float64(time.Microsecond))

	return s.Collect(ctx)
}
