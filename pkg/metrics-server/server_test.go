package metric_server

import (
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	storage "sigs.k8s.io/metrics-server/pkg/storage"
)

func TestServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics server Suite")
}

var _ = Describe("Metrics server", func() {
	var (
		server *MetricsServer
		s      *storage.Storage
		req    *http.Request
	)

	BeforeEach(func() {
		s = storage.NewStorage()
		server = &MetricsServer{resolution: 60 * time.Second, storage: s}
	})

	It("liveness should fail when last scrape failed", func() {
		By("last scrape was 2 minutes ago")
		server.lastTickStart = time.Now().Add(-2 * time.Minute)
		err := server.CheckLiveness(req)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).Should(ContainSubstring("was greater than expected metrics resolution"))
	})

	It("liveness should fail when storage is empty", func() {
		By("last scrape was succcesful")
		server.lastTickStart = time.Now().Add(1 * time.Minute)
		err := server.CheckLiveness(req)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).Should(ContainSubstring("no metrics available in storage cache"))
	})

	It("readines error when storage is empty", func() {
		By("Checking for readiness")
		err := server.CheckReadiness(req)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).Should(ContainSubstring("no metrics available in storage cache"))
	})

})
