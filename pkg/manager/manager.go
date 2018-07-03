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
	"time"

	"github.com/kubernetes-incubator/metrics-server/pkg/sink"
	"github.com/kubernetes-incubator/metrics-server/pkg/sources"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	DefaultScrapeOffset   = 5 * time.Second
	DefaultMaxParallelism = 3
)

var (
	// The Time spent in a processor in microseconds.
	processorDuration = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: "heapster",
			Subsystem: "processor",
			Name:      "duration_microseconds",
			Help:      "The Time spent in a processor in microseconds.",
		},
		[]string{"processor"},
	)
)

func init() {
	prometheus.MustRegister(processorDuration)
}

// Runnable represents something that can be run until a signal is given to stop.
type Runnable interface {
	// Run runs this runnable until the given channel is closed.
	// It should not block -- it will spawn its own goroutine.
	RunUntil(stopCh <-chan struct{})
}

type realManager struct {
	source                 sources.MetricSource
	sink                   sink.MetricSink
	resolution             time.Duration
	scrapeOffset           time.Duration
	housekeepSemaphoreChan chan struct{}
	housekeepTimeout       time.Duration
}

func NewManager(metricSrc sources.MetricSource, metricSink sink.MetricSink, resolution time.Duration,
	scrapeOffset time.Duration, maxParallelism int) Runnable {
	manager := realManager{
		source:           metricSrc,
		sink:             metricSink,
		resolution:       resolution,
		scrapeOffset:     scrapeOffset,
		housekeepTimeout: resolution / 2,
	}

	return &manager
}

func (rm *realManager) RunUntil(stopCh <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(rm.resolution)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				data, err := rm.source.Collect()
				if err != nil {
					glog.Errorf("unable to collect metrics: %v", err)
					continue
				}
				err = rm.sink.Receive(data)
				if err != nil {
					glog.Errorf("unable to save metrics: %v", err)
				}
			case <-stopCh:
				return
			}
		}
	}()
}
