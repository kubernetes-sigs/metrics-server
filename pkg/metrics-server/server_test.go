package metric_server

import (
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics server Suite")
}

var _ = Describe("Metrics server", func() {
	var (
		server *MetricsServer
		req    *http.Request
	)

	BeforeEach(func() {
		server = &MetricsServer{resolution: 60 * time.Second}
	})

	It("liveness should fail when last scrape didnt happen on the resolution ticker ", func() {
		By("last scrape was 2 minutes ago")
		server.lastTickStart = time.Now().Add(-2 * time.Minute)
		err := server.CheckLiveness(req)
		Expect(err).ToNot(BeNil())
		Expect(err.Error()).Should(ContainSubstring("was greater than expected metrics resolution"))
	})
})
