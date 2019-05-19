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

	"github.com/prometheus/client_golang/prometheus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog"

	utilmetrics "github.com/kubernetes-incubator/metrics-server/pkg/metrics"
)

const (
	maxDelayMs       = 4 * 1000
	delayPerSourceMs = 8
)

var (
	lastScrapeTimestamp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
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
	scraperDuration *prometheus.HistogramVec = prometheus.NewHistogramVec(prometheus.HistogramOpts{}, []string{"source"})
)

func init() {
	prometheus.MustRegister(lastScrapeTimestamp)
}

// RegisterScraperDuration creates and registers a histogram metric for
// scrape duration, suitable for use in the source manager.
func RegisterDurationMetrics(scrapeTimeout time.Duration) {
	scraperDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "metrics_server",
			Subsystem: "scraper",
			Name:      "duration_seconds",
			Help:      "Time spent scraping sources in seconds.",
			Buckets:   utilmetrics.BucketsForScrapeDuration(scrapeTimeout),
		},
		[]string{"source"},
	)
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
	klog.V(1).Infof("Scraping metrics from %v sources", len(sources))

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

			klog.V(2).Infof("Querying source: %s", source)
			metrics, err := scrapeWithMetrics(ctx, source)
			if err != nil {
				err = fmt.Errorf("unable to fully scrape metrics from source %s: %v", source.Name(), err)
			}
			responseChannel <- metrics
			errChannel <- err
		}(source)
	}

	res := &MetricsBatch{}

	for range sources {
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

	klog.V(1).Infof("ScrapeMetrics: time: %s, nodes: %v, pods: %v", time.Since(startTime), len(res.Nodes), len(res.Pods))
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
		Observe(float64(time.Since(startTime)) / float64(time.Second))

	return s.Collect(ctx)
}
