package server

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/metrics/pkg/apis/metrics"

	"sigs.k8s.io/metrics-server/pkg/api"
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
				Nodes: []storage.NodeMetricsPoint{
					{
						Name: "node1",
						MetricsPoint: storage.MetricsPoint{
							Timestamp:   time.Now(),
							CpuUsage:    resource.Quantity{},
							MemoryUsage: resource.Quantity{},
						},
					},
				},
			},
		}
		store = &storageMock{}
		server = NewServer(nil, nil, nil, store, scraper, resolution)
	})

	It("liveness should pass before first scrape tick finishes", func() {
		Expect(server.CheckLiveness(nil)).To(Succeed())
	})
	It("liveness should pass if scrape fails", func() {
		scraper.err = fmt.Errorf("failed to scrape")
		server.tick(context.Background(), time.Now())
		Expect(server.CheckLiveness(nil)).To(Succeed())
	})
	It("liveness should pass if scrape succeeds", func() {
		server.tick(context.Background(), time.Now().Add(-resolution))
		Expect(server.CheckLiveness(nil)).To(Succeed())
	})
	It("liveness should fail if last scrape took longer then expected", func() {
		server.tick(context.Background(), time.Now().Add(-2*resolution))
		Expect(server.CheckLiveness(nil)).NotTo(Succeed())
	})
	It("readiness should fail before first tick finishes", func() {
		Expect(server.CheckReadiness(nil)).To(Succeed())
	})
	It("readiness should pass if scrape succeeds", func() {
		server.tick(context.Background(), time.Now())
		Expect(server.CheckReadiness(nil)).To(Succeed())
	})
	It("readiness should pass if scrape returns empty result", func() {
		scraper.result.Nodes = []storage.NodeMetricsPoint{}
		server.tick(context.Background(), time.Now())
		Expect(server.CheckReadiness(nil)).To(Succeed())
	})
	It("readiness should pass if scrape fails but returns at least one result", func() {
		scraper.err = fmt.Errorf("failed to scrape")
		server.tick(context.Background(), time.Now())
		Expect(server.CheckReadiness(nil)).To(Succeed())
	})
	It("readiness should fail if scrape fails without results", func() {
		scraper.err = fmt.Errorf("failed to scrape")
		scraper.result.Nodes = []storage.NodeMetricsPoint{}
		server.tick(context.Background(), time.Now())
		Expect(server.CheckReadiness(nil)).NotTo(Succeed())
	})
})

type scraperMock struct {
	result *storage.MetricsBatch
	err    error
}

var _ scraper.Scraper = (*scraperMock)(nil)

func (s *scraperMock) Scrape(ctx context.Context) (*storage.MetricsBatch, error) {
	return s.result, s.err
}

type storageMock struct{}

var _ storage.Storage = (*storageMock)(nil)

func (s *storageMock) Store(batch *storage.MetricsBatch) {}

func (s *storageMock) GetContainerMetrics(pods ...apitypes.NamespacedName) ([]api.TimeInfo, [][]metrics.ContainerMetrics) {
	return nil, nil
}

func (s *storageMock) GetNodeMetrics(nodes ...string) ([]api.TimeInfo, []corev1.ResourceList) {
	return nil, nil
}
