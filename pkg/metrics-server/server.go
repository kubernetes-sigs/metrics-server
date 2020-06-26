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

package metric_server

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
	"k8s.io/klog"

	"sigs.k8s.io/metrics-server/pkg/scraper"
	"sigs.k8s.io/metrics-server/pkg/storage"
	"sigs.k8s.io/metrics-server/pkg/utils"
)

var (
	// initialized below to an actual value by a call to RegisterTickDuration
	// (acts as a no-op by default), but we can't just register it in the constructor,
	// since it could be called multiple times during setup.
	tickDuration *metrics.Histogram = metrics.NewHistogram(&metrics.HistogramOpts{})
)

// RegisterTickDuration creates and registers a histogram metric for
// scrape duration.
func RegisterServerMetrics(resolution time.Duration) {
	tickDuration = metrics.NewHistogram(
		&metrics.HistogramOpts{
			Namespace: "metrics_server",
			Subsystem: "manager",
			Name:      "tick_duration_seconds",
			Help:      "The total time spent collecting and storing metrics in seconds.",
			Buckets:   utils.BucketsForScrapeDuration(resolution),
		},
	)
	legacyregistry.MustRegister(tickDuration)
}

// MetricsServer scrapes metrics and serves then using k8s api.
type MetricsServer struct {
	*genericapiserver.GenericAPIServer

	syncs    []cache.InformerSynced
	informer informers.SharedInformerFactory

	storage    *storage.Storage
	scraper    *scraper.Scraper
	resolution time.Duration

	healthMu      sync.RWMutex
	lastTickStart time.Time
	lastOk        bool
	healthyScrape bool
}

// RunUntil starts background scraping goroutine and runs apiserver serving metrics.
func (ms *MetricsServer) RunUntil(stopCh <-chan struct{}) error {
	ms.informer.Start(stopCh)
	shutdown := cache.WaitForCacheSync(stopCh, ms.syncs...)
	if !shutdown {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go ms.runScrape(ctx)
	return ms.GenericAPIServer.PrepareRun().Run(stopCh)
}

func (ms *MetricsServer) runScrape(ctx context.Context) {
	ticker := time.NewTicker(ms.resolution)
	defer ticker.Stop()
	ms.scrape(ctx, time.Now())

	for {
		select {
		case startTime := <-ticker.C:
			ms.scrape(ctx, startTime)
		case <-ctx.Done():
			return
		}
	}
}

func (ms *MetricsServer) scrape(ctx context.Context, startTime time.Time) {
	ms.healthMu.Lock()
	ms.lastTickStart = startTime
	ms.healthMu.Unlock()

	healthyScrape := true
	ctx, cancelTimeout := context.WithTimeout(ctx, ms.resolution)
	defer cancelTimeout()

	klog.V(6).Infof("Beginning cycle, scraping metrics...")
	data, scrapeErr := ms.scraper.Scrape(ctx)
	if scrapeErr != nil {
		klog.Errorf("unable to fully scrape metrics: %v", scrapeErr)

		// only consider this an indication of unhealthy scrape if we
		// couldn't scrape from any nodes -- one node going down
		// shouldn't indicate that metrics-server is unhealthy
		if len(data.Nodes) == 0 {
			healthyScrape = false
		}
		// NB: continue on so that we don't lose all metrics
		// if one node goes down
	}

	klog.V(6).Infof("...Storing metrics...")
	recvErr := ms.storage.Store(data)
	if recvErr != nil {
		klog.Errorf("unable to save metrics: %v", recvErr)
		// any failure to save means unhealthy scrape
		healthyScrape = false
	}

	collectTime := time.Since(startTime)
	tickDuration.Observe(float64(collectTime) / float64(time.Second))
	klog.V(6).Infof("...Cycle complete")

	ms.healthMu.Lock()
	ms.lastOk = true
	ms.healthyScrape = healthyScrape
	ms.healthMu.Unlock()

}

// Check if MS is alive by looking at last tick time and checking if
// last scrape completed
// Serves the Liveness probe
func (ms *MetricsServer) CheckLiveness(_ *http.Request) error {
	ms.healthMu.RLock()
	lastTick := ms.lastTickStart
	lastOk := ms.lastOk
	ms.healthMu.RUnlock()

	maxTickWait := time.Duration(1.1 * float64(ms.resolution))
	tickWait := time.Since(lastTick)
	if tickWait > maxTickWait {
		return fmt.Errorf("time since last tick (%s) was greater than expected metrics resolution (%s)", tickWait, maxTickWait)
	}
	if !lastOk {
		return fmt.Errorf("last scrape didnt complete")
	}
	return nil
}

// Check if MS is ready by checking if last scrape completed
// if we have at least one node in the collected data.
// Serves the Readiness probe
func (ms *MetricsServer) CheckReadiness(_ *http.Request) error {
	//TODO[Hanu] ping apiserver for its availability
	// https://github.com/kubernetes/apiserver/blob/42312e1d6801a8741504db78292773e9aa141bd8/pkg/server/healthz/healthz.go#L52
	ms.healthMu.RLock()
	healthyScrape := ms.healthyScrape
	lastOk := ms.lastOk
	ms.healthMu.RUnlock()

	if !lastOk {
		return fmt.Errorf("last scrape didnt complete")
	}
	if !healthyScrape {
		return fmt.Errorf("last scrape wasn't healthy")
	}
	return nil
}
