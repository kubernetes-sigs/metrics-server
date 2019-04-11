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

package manager

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	utilmetrics "github.com/kubernetes-incubator/metrics-server/pkg/metrics"
	"github.com/kubernetes-incubator/metrics-server/pkg/sink"
	"github.com/kubernetes-incubator/metrics-server/pkg/sources"
	"k8s.io/klog"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// initialized below to an actual value by a call to RegisterTickDuration
	// (acts as a no-op by default), but we can't just register it in the constructor,
	// since it could be called multiple times during setup.
	tickDuration prometheus.Histogram = prometheus.NewHistogram(prometheus.HistogramOpts{})
)

// RegisterTickDuration creates and registers a histogram metric for
// scrape duration, suitable for use in the overall manager.
func RegisterDurationMetrics(resolution time.Duration) {
	tickDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "metrics_server",
			Subsystem: "manager",
			Name:      "tick_duration_seconds",
			Help:      "The total time spent collecting and storing metrics in seconds.",
			Buckets:   utilmetrics.BucketsForScrapeDuration(resolution),
		},
	)
	prometheus.MustRegister(tickDuration)
}

// Runnable represents something that can be run until a signal is given to stop.
type Runnable interface {
	// Run runs this runnable until the given channel is closed.
	// It should not block -- it will spawn its own goroutine.
	RunUntil(stopCh <-chan struct{})
}

type Manager struct {
	source     sources.MetricSource
	sink       sink.MetricSink
	resolution time.Duration

	healthMu      sync.RWMutex
	lastTickStart time.Time
	lastOk        bool
}

func NewManager(metricSrc sources.MetricSource, metricSink sink.MetricSink, resolution time.Duration) *Manager {
	manager := Manager{
		source:     metricSrc,
		sink:       metricSink,
		resolution: resolution,
	}

	return &manager
}

func (rm *Manager) RunUntil(stopCh <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(rm.resolution)
		defer ticker.Stop()
		rm.Collect(time.Now())

		for {
			select {
			case startTime := <-ticker.C:
				rm.Collect(startTime)
			case <-stopCh:
				return
			}
		}
	}()
}

func (rm *Manager) Collect(startTime time.Time) {
	rm.healthMu.Lock()
	rm.lastTickStart = startTime
	rm.healthMu.Unlock()

	healthyTick := true

	ctx, cancelTimeout := context.WithTimeout(context.Background(), rm.resolution)
	defer cancelTimeout()

	klog.V(6).Infof("Beginning cycle, collecting metrics...")
	data, collectErr := rm.source.Collect(ctx)
	if collectErr != nil {
		klog.Errorf("unable to fully collect metrics: %v", collectErr)

		// only consider this an indication of bad health if we
		// couldn't collect from any nodes -- one node going down
		// shouldn't indicate that metrics-server is unhealthy
		if len(data.Nodes) == 0 {
			healthyTick = false
		}

		// NB: continue on so that we don't lose all metrics
		// if one node goes down
	}

	klog.V(6).Infof("...Storing metrics...")
	recvErr := rm.sink.Receive(data)
	if recvErr != nil {
		klog.Errorf("unable to save metrics: %v", recvErr)

		// any failure to save means we're unhealthy
		healthyTick = false
	}

	collectTime := time.Now().Sub(startTime)
	tickDuration.Observe(float64(collectTime) / float64(time.Second))
	klog.V(6).Infof("...Cycle complete")

	rm.healthMu.Lock()
	rm.lastOk = healthyTick
	rm.healthMu.Unlock()
}

// CheckHealth checks the health of the manager by looking at tick times,
// and checking if we have at least one node in the collected data.
// It implements the health checker func part of the healthz checker.
func (rm *Manager) CheckHealth(_ *http.Request) error {
	rm.healthMu.RLock()
	lastTick := rm.lastTickStart
	healthyTick := rm.lastOk
	rm.healthMu.RUnlock()

	// use 1.1 for a bit of wiggle room
	maxTickWait := time.Duration(1.1 * float64(rm.resolution))
	tickWait := time.Now().Sub(lastTick)

	if tickWait > maxTickWait {
		return fmt.Errorf("time since last tick (%s) was greater than expected metrics resolution (%s)", tickWait, maxTickWait)
	}

	if !healthyTick {
		return fmt.Errorf("there was an error collecting or saving metrics in the last collection tick")
	}

	return nil
}
