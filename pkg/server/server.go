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

package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/oklog/run"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/client-go/tools/cache"
	"k8s.io/component-base/metrics"
	"k8s.io/klog/v2"

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

// RegisterServerMetrics creates and registers a histogram metric for
// scrape duration.
func RegisterServerMetrics(registrationFunc func(metrics.Registerable) error, resolution time.Duration) error {
	tickDuration = metrics.NewHistogram(
		&metrics.HistogramOpts{
			Namespace: "metrics_server",
			Subsystem: "manager",
			Name:      "tick_duration_seconds",
			Help:      "The total time spent collecting and storing metrics in seconds.",
			Buckets:   utils.BucketsForScrapeDuration(resolution),
		},
	)
	return registrationFunc(tickDuration)
}

func NewServer(
	nodes cache.Controller,
	pods cache.Controller,
	apiserver *genericapiserver.GenericAPIServer,
	metricsHandler http.HandlerFunc,
	metricsAddress string,
	storage storage.Storage,
	scraper scraper.Scraper, resolution time.Duration,
) *server {
	return &server{
		nodes:            nodes,
		pods:             pods,
		GenericAPIServer: apiserver,
		metricsHandler:   metricsHandler,
		metricsAddress:   metricsAddress,
		storage:          storage,
		scraper:          scraper,
		resolution:       resolution,
	}
}

// server scrapes metrics and serves then using k8s api.
type server struct {
	*genericapiserver.GenericAPIServer
	metricsHandler http.HandlerFunc
	metricsAddress string

	pods  cache.Controller
	nodes cache.Controller

	storage    storage.Storage
	scraper    scraper.Scraper
	resolution time.Duration

	// tickStatusMux protects tick fields
	tickStatusMux sync.RWMutex
	// tickLastStart is equal to start time of last unfinished tick
	tickLastStart time.Time
}

// RunUntil starts background scraping goroutine and runs apiserver serving metrics.
func (s *server) RunUntil(stopCh <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start informers
	go s.nodes.Run(stopCh)
	go s.pods.Run(stopCh)

	// Ensure cache is up to date
	ok := cache.WaitForCacheSync(stopCh, s.nodes.HasSynced)
	if !ok {
		return nil
	}
	ok = cache.WaitForCacheSync(stopCh, s.pods.HasSynced)
	if !ok {
		return nil
	}

	g := run.Group{}
	s.addRunScrape(ctx, &g)
	s.addGenericAPIServer(stopCh, cancel, &g)
	if s.metricsAddress != "" {
		s.addMetricsServer(ctx, &g)
	}

	return g.Run()
}

func (s *server) newMetricsServer() *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", s.metricsHandler)
	return &http.Server{
		Addr:    s.metricsAddress,
		Handler: mux,
	}
}

func (s *server) runScrape(ctx context.Context) {
	ticker := time.NewTicker(s.resolution)
	defer ticker.Stop()
	s.tick(ctx, time.Now())

	for {
		select {
		case startTime := <-ticker.C:
			s.tick(ctx, startTime)
		case <-ctx.Done():
			return
		}
	}
}

func (s *server) tick(ctx context.Context, startTime time.Time) {
	s.tickStatusMux.Lock()
	s.tickLastStart = startTime
	s.tickStatusMux.Unlock()

	ctx, cancelTimeout := context.WithTimeout(ctx, s.resolution)
	defer cancelTimeout()

	klog.V(6).InfoS("Scraping metrics")
	data := s.scraper.Scrape(ctx)

	klog.V(6).InfoS("Storing metrics")
	s.storage.Store(data)

	collectTime := time.Since(startTime)
	tickDuration.Observe(float64(collectTime) / float64(time.Second))
	klog.V(6).InfoS("Scraping cycle complete")
}

func (s *server) addRunScrape(ctx context.Context, g *run.Group) {
	ctx, cancel := context.WithCancel(ctx)
	g.Add(func() error {
		s.runScrape(ctx)
		return nil
	}, func(err error) {
		cancel()
	})
}

func (s *server) addMetricsServer(ctx context.Context, g *run.Group) {
	ctx, cancel := context.WithCancel(ctx)
	metricsServer := s.newMetricsServer()
	g.Add(func() error {
		return metricsServer.ListenAndServe()
	}, func(err error) {
		klog.InfoS("Shutting down metrics handler")
		if err := metricsServer.Shutdown(ctx); err != nil {
			klog.ErrorS(err, "Could not gracefully shut down metrics handler")
		}
		cancel()
	})
}

func (s *server) addGenericAPIServer(stopCh <-chan struct{}, cancel context.CancelFunc, g *run.Group) {
	g.Add(func() error {
		return s.GenericAPIServer.PrepareRun().Run(stopCh)
	}, func(err error) {
		cancel()
	})
}

func (s *server) RegisterProbes(waiter cacheSyncWaiter) error {
	err := s.AddReadyzChecks(s.probeMetricStorageReady("metric-storage-ready"))
	if err != nil {
		return err
	}
	err = s.AddLivezChecks(0, s.probeMetricCollectionTimely("metric-collection-timely"))
	if err != nil {
		return err
	}
	err = s.AddHealthChecks(MetadataInformerSyncHealthz("metadata-informer-sync", waiter))
	if err != nil {
		return err
	}
	return nil
}

// Check if MS is alive by looking at last tick time.
// If its deadlock or panic, tick wouldn't be happening on the tick interval
func (s *server) probeMetricCollectionTimely(name string) healthz.HealthChecker {
	return healthz.NamedCheck(name, func(_ *http.Request) error {
		s.tickStatusMux.RLock()
		tickLastStart := s.tickLastStart
		s.tickStatusMux.RUnlock()

		maxTickWait := time.Duration(1.5 * float64(s.resolution))
		tickWait := time.Since(tickLastStart)
		if !tickLastStart.IsZero() && tickWait > maxTickWait {
			err := fmt.Errorf("metric collection didn't finish on time")
			klog.InfoS("Failed probe", "probe", name, "err", err, "duration", tickWait, "maxDuration", maxTickWait)
			return err
		}
		return nil
	})
}

// Check if MS is ready by checking if last tick was ok
func (s *server) probeMetricStorageReady(name string) healthz.HealthChecker {
	return healthz.NamedCheck(name, func(r *http.Request) error {
		if !s.storage.Ready() {
			err := fmt.Errorf("no metrics to serve")
			klog.InfoS("Failed probe", "probe", name, "err", err)
			return err
		}
		return nil
	})
}
