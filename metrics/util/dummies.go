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

package util

import (
	"sync"
	"time"

	"github.com/kubernetes-incubator/metrics-server/metrics/core"
)

type DummySink struct {
	name        string
	mutex       sync.Mutex
	exportCount int
	stopped     bool
	latency     time.Duration
}

func (s *DummySink) Name() string {
	return s.name
}
func (s *DummySink) ExportData(*core.DataBatch) {
	s.mutex.Lock()
	s.exportCount++
	s.mutex.Unlock()

	time.Sleep(s.latency)
}

func (s *DummySink) Stop() {
	s.mutex.Lock()
	s.stopped = true
	s.mutex.Unlock()

	time.Sleep(s.latency)
}

func (s *DummySink) IsStopped() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.stopped
}

func (s *DummySink) GetExportCount() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.exportCount
}

func NewDummySink(name string, latency time.Duration) *DummySink {
	return &DummySink{
		name:        name,
		latency:     latency,
		exportCount: 0,
		stopped:     false,
	}
}

type DummyMetricsSource struct {
	latency   time.Duration
	metricSet core.MetricSet
}

func (s *DummyMetricsSource) Name() string {
	return "dummy"
}

func (s *DummyMetricsSource) ScrapeMetrics(start, end time.Time) *core.DataBatch {
	time.Sleep(s.latency)
	return &core.DataBatch{
		Timestamp: end,
		MetricSets: map[string]*core.MetricSet{
			s.metricSet.Labels["name"]: &s.metricSet,
		},
	}
}

func newDummyMetricSet(name string) core.MetricSet {
	return core.MetricSet{
		MetricValues: map[string]core.MetricValue{},
		Labels: map[string]string{
			"name": name,
		},
	}
}

func NewDummyMetricsSource(name string, latency time.Duration) *DummyMetricsSource {
	return &DummyMetricsSource{
		latency:   latency,
		metricSet: newDummyMetricSet(name),
	}
}

type DummyMetricsSourceProvider struct {
	sources []core.MetricsSource
}

func (p *DummyMetricsSourceProvider) GetMetricsSources() []core.MetricsSource {
	return p.sources
}

func NewDummyMetricsSourceProvider(sources ...core.MetricsSource) *DummyMetricsSourceProvider {
	return &DummyMetricsSourceProvider{
		sources: sources,
	}
}

type DummyDataProcessor struct {
	latency time.Duration
}

func (p *DummyDataProcessor) Name() string {
	return "dummy"
}

func (p *DummyDataProcessor) Process(data *core.DataBatch) (*core.DataBatch, error) {
	time.Sleep(p.latency)
	return data, nil
}

func NewDummyDataProcessor(latency time.Duration) *DummyDataProcessor {
	return &DummyDataProcessor{
		latency: latency,
	}
}
