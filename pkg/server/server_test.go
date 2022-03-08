// Copyright 2020 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
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
	"testing"
	"time"

	"k8s.io/metrics/pkg/apis/metrics"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/metrics-server/pkg/scraper"
	"sigs.k8s.io/metrics-server/pkg/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Suite")
}

var _ = Describe("Server", func() {
	var (
		resolution time.Duration
		server     *server
		scraper    *scraperMock
		store      *storageMock
	)

	BeforeEach(func() {
		resolution = 60 * time.Second
		scraper = &scraperMock{
			result: &storage.MetricsBatch{
				Nodes: map[string]storage.MetricsPoint{
					"node1": {
						Timestamp:         time.Now(),
						CumulativeCpuUsed: 0,
						MemoryUsage:       0,
					},
				},
			},
		}
		store = &storageMock{}
		server = NewServer(nil, nil, nil, store, scraper, resolution)
	})

	It("metric-collection-timely probe should pass before first scrape tick finishes", func() {
		check := server.probeMetricCollectionTimely("")
		Expect(check.Check(nil)).To(Succeed())
	})
	It("metric-collection-timely probe should pass if scrape fails", func() {
		scraper.err = fmt.Errorf("failed to scrape")
		server.tick(context.Background(), time.Now())
		check := server.probeMetricCollectionTimely("")
		Expect(check.Check(nil)).To(Succeed())
	})
	It("metric-collection-timely probe should pass if scrape succeeds", func() {
		server.tick(context.Background(), time.Now().Add(-resolution))
		check := server.probeMetricCollectionTimely("")
		Expect(check.Check(nil)).To(Succeed())
	})
	It("metric-collection-timely probe should fail if last scrape took longer than expected", func() {
		server.tick(context.Background(), time.Now().Add(-2*resolution))
		check := server.probeMetricCollectionTimely("")
		Expect(check.Check(nil)).NotTo(Succeed())
	})
	It("metric-storage-ready probe should fail if store is not ready", func() {
		check := server.probeMetricStorageReady("")
		Expect(check.Check(nil)).NotTo(Succeed())
	})
	It("metric-storage-ready probe should pass if store is ready", func() {
		store.ready = true
		check := server.probeMetricStorageReady("")
		Expect(check.Check(nil)).To(Succeed())
	})
})

type scraperMock struct {
	result *storage.MetricsBatch
	err    error
}

var _ scraper.Scraper = (*scraperMock)(nil)

func (s *scraperMock) Scrape(ctx context.Context) *storage.MetricsBatch {
	return s.result
}

type storageMock struct {
	ready bool
}

var _ storage.Storage = (*storageMock)(nil)

func (s *storageMock) Store(batch *storage.MetricsBatch) {}

func (s *storageMock) GetPodMetrics(pods ...*metav1.PartialObjectMetadata) ([]metrics.PodMetrics, error) {
	return nil, nil
}

func (s *storageMock) GetNodeMetrics(nodes ...*corev1.Node) ([]metrics.NodeMetrics, error) {
	return nil, nil
}

func (s *storageMock) Ready() bool {
	return s.ready
}
