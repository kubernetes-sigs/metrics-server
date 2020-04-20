package test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/prometheus/common/expfmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/transport"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

func TestMetricsServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "[MetricsServer]")
}

var _ = Describe("MetricsServer", func() {
	restConfig, err := getRestConfig()
	if err != nil {
		panic(err)
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		panic(err)
	}
	mclient, err := metricsclientset.NewForConfig(restConfig)
	if err != nil {
		panic(err)
	}
	It("exposes metrics from at least one pod in cluster", func() {
		podMetrics, err := mclient.MetricsV1beta1().PodMetricses(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to list pod metrics")
		Expect(podMetrics.Items).NotTo(BeEmpty(), "Need at least one pod to verify if MetricsServer works")
	})
	It("exposes metrics about all nodes in cluster", func() {
		nodeList, err := client.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			panic(err)
		}
		Expect(nodeList.Items).NotTo(BeEmpty(), "Need at least one node to verify if MetricsServer works")
		for _, node := range nodeList.Items {
			_, err := mclient.MetricsV1beta1().NodeMetricses().Get(context.Background(), node.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "Metrics for node %s are not available", node.Name)
		}
	})
	It("exposes prometheus metrics", func() {
		podList, err := client.CoreV1().Pods(metav1.NamespaceSystem).List(context.Background(), metav1.ListOptions{LabelSelector: "k8s-app=metrics-server"})
		Expect(err).NotTo(HaveOccurred(), "Failed to find Metrics Server pod")
		Expect(podList.Items).NotTo(BeEmpty(), "Metrics Server pod was not found")
		Expect(podList.Items).To(HaveLen(1), "Expect to only have one Metrics Server pod")
		msPod := podList.Items[0]
		resp, err := proxyRequestToPod(restConfig, msPod.Namespace, msPod.Name, "https", 4443, "/metrics")
		Expect(err).NotTo(HaveOccurred(), "Failed to get Metrics Server /metrics endpoint")
		metrics, err := parseMetricNames(resp)
		Expect(err).NotTo(HaveOccurred(), "Failed to parse Metrics Server metrics")
		sort.Strings(metrics)

		Expect(metrics).To(Equal([]string{
			"apiserver_audit_event_total",
			"apiserver_audit_requests_rejected_total",
			"apiserver_client_certificate_expiration_seconds",
			"apiserver_current_inflight_requests",
			"apiserver_envelope_encryption_dek_cache_fill_percent",
			"apiserver_request_duration_seconds",
			"apiserver_request_total",
			"apiserver_response_sizes",
			"apiserver_storage_data_key_generation_duration_seconds",
			"apiserver_storage_data_key_generation_failures_total",
			"apiserver_storage_envelope_transformation_cache_misses_total",
			"authenticated_user_requests",
			"authentication_attempts",
			"authentication_duration_seconds",
			"go_gc_duration_seconds",
			"go_goroutines",
			"go_info",
			"go_memstats_alloc_bytes",
			"go_memstats_alloc_bytes_total",
			"go_memstats_buck_hash_sys_bytes",
			"go_memstats_frees_total",
			"go_memstats_gc_cpu_fraction",
			"go_memstats_gc_sys_bytes",
			"go_memstats_heap_alloc_bytes",
			"go_memstats_heap_idle_bytes",
			"go_memstats_heap_inuse_bytes",
			"go_memstats_heap_objects",
			"go_memstats_heap_released_bytes",
			"go_memstats_heap_sys_bytes",
			"go_memstats_last_gc_time_seconds",
			"go_memstats_lookups_total",
			"go_memstats_mallocs_total",
			"go_memstats_mcache_inuse_bytes",
			"go_memstats_mcache_sys_bytes",
			"go_memstats_mspan_inuse_bytes",
			"go_memstats_mspan_sys_bytes",
			"go_memstats_next_gc_bytes",
			"go_memstats_other_sys_bytes",
			"go_memstats_stack_inuse_bytes",
			"go_memstats_stack_sys_bytes",
			"go_memstats_sys_bytes",
			"go_threads",
			"metrics_server_kubelet_summary_request_duration_seconds",
			"metrics_server_kubelet_summary_scrapes_total",
			"metrics_server_manager_tick_duration_seconds",
			"metrics_server_scraper_duration_seconds",
			"metrics_server_scraper_last_time_seconds",
			"process_cpu_seconds_total",
			"process_max_fds",
			"process_open_fds",
			"process_resident_memory_bytes",
			"process_start_time_seconds",
			"process_virtual_memory_bytes",
			"process_virtual_memory_max_bytes",
		}), "Unexpected metrics")
	})
})

func getRestConfig() (*rest.Config, error) {
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return nil, err
	}
	return clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
}

func parseMetricNames(data []byte) ([]string, error) {
	var parser expfmt.TextParser
	mfs, err := parser.TextToMetricFamilies(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	Expect(err).NotTo(HaveOccurred(), "Failed to parse mfs")
	var ms []string
	for key := range mfs {
		ms = append(ms, key)
	}
	return ms, nil
}

func proxyRequestToPod(config *rest.Config, namespace, podname, scheme string, port int, path string) ([]byte, error) {
	cancel, err := setupForwarding(config, namespace, podname)
	defer cancel()
	if err != nil {
		return nil, err
	}

	reqUrl := url.URL{Scheme: scheme, Path: path, Host: fmt.Sprintf("127.0.0.1:%d", port)}
	resp, err := sendRequest(config, reqUrl.String())
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func setupForwarding(config *rest.Config, namespace, podname string) (cancel func(), err error) {
	hostIP := strings.TrimLeft(config.Host, "https://")

	trans, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return noop, err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: trans}, http.MethodPost, &url.URL{Scheme: "https", Path: fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, podname), Host: hostIP})

	var berr, bout bytes.Buffer
	buffErr := bufio.NewWriter(&berr)
	buffOut := bufio.NewWriter(&bout)

	stopCh := make(chan struct{})
	readyCh := make(chan struct{})

	fw, err := portforward.New(dialer, []string{"4443:4443"}, stopCh, readyCh, buffOut, buffErr)
	if err != nil {
		return noop, err
	}
	go func() {
		fmt.Print(fw.ForwardPorts())
	}()
	<-readyCh
	return func() {
		stopCh <- struct{}{}
	}, nil
}

func sendRequest(config *rest.Config, url string) (*http.Response, error) {
	tsConfig, err := config.TransportConfig()
	if err != nil {
		return nil, err
	}
	tsConfig.TLS.Insecure = true
	tsConfig.TLS.CAData = []byte{}

	ts, err := transport.New(tsConfig)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Transport: ts}
	return client.Get(url)
}

func noop() {}
