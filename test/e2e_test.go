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

package test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/expfmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

const (
	localPort                    = 10250
	cpuConsumerPodName           = "cpu-consumer"
	memoryConsumerPodName        = "memory-consumer"
	initContainerPodName         = "cmwithinitcontainer-consumer"
	sideCarContainerPodName      = "sidecarpod-consumer"
	initSidecarContainersPodName = "initsidecarpod-consumer"
	labelSelector                = "metrics-server-skip!=true"
	skipLabel                    = "metrics-server-skip==true"
	labelKey                     = "metrics-server-skip"
)

var (
	client                 *clientset.Clientset
	testSideCarsContainers bool
)

func TestMetricsServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "[MetricsServer]")
}

var _ = BeforeSuite(func() {
	deletePod(client, cpuConsumerPodName)
	err := consumeCPU(client, cpuConsumerPodName, labelKey)
	if err != nil {
		panic(err)
	}
	deletePod(client, memoryConsumerPodName)
	err = consumeMemory(client, memoryConsumerPodName, labelKey)
	if err != nil {
		panic(err)
	}
	deletePod(client, initContainerPodName)
	err = consumeWithInitContainer(client, initContainerPodName, labelKey)
	if err != nil {
		panic(err)
	}
	deletePod(client, sideCarContainerPodName)
	err = consumeWithSideCarContainer(client, sideCarContainerPodName, labelKey)
	if err != nil {
		panic(err)
	}
	if testSideCarsContainers {
		deletePod(client, initSidecarContainersPodName)
		err = consumeWithInitSideCarContainer(client, initSidecarContainersPodName, labelKey)
		if err != nil {
			panic(err)
		}
	}
})

var _ = AfterSuite(func() {
	deletePod(client, cpuConsumerPodName)
	deletePod(client, memoryConsumerPodName)
	deletePod(client, initContainerPodName)
	deletePod(client, sideCarContainerPodName)
	deletePod(client, initSidecarContainersPodName)
})

var _ = Describe("MetricsServer", func() {
	restConfig, err := getRestConfig()
	if err != nil {
		panic(err)
	}
	client, err = clientset.NewForConfig(restConfig)
	if err != nil {
		panic(err)
	}
	mclient, err := metricsclientset.NewForConfig(restConfig)
	if err != nil {
		panic(err)
	}

	testSideCarsContainers = hasSidecarFeatureEnabled(client)

	It("exposes metrics from at least one pod in cluster", func() {
		podMetrics, err := mclient.MetricsV1beta1().PodMetricses(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred(), "Failed to list pod metrics")
		Expect(podMetrics.Items).NotTo(BeEmpty(), "Need at least one pod to verify if MetricsServer works")
	})
	It("exposes metrics about all nodes in cluster", func() {
		nodeList, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			panic(err)
		}
		Expect(nodeList.Items).NotTo(BeEmpty(), "Need at least one node to verify if MetricsServer works")
		for _, node := range nodeList.Items {
			_, err := mclient.MetricsV1beta1().NodeMetricses().Get(context.TODO(), node.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "Metrics for node %s are not available", node.Name)
		}
	})
	It("returns accurate CPU metric", func() {
		Expect(err).NotTo(HaveOccurred(), "Failed to create %q pod", cpuConsumerPodName)
		deadline := time.Now().Add(60 * time.Second)
		var ms *v1beta1.PodMetrics
		for {
			ms, err = mclient.MetricsV1beta1().PodMetricses(metav1.NamespaceDefault).Get(context.TODO(), cpuConsumerPodName, metav1.GetOptions{})
			if err == nil || time.Now().After(deadline) {
				break
			}
			time.Sleep(5 * time.Second)
		}
		Expect(err).NotTo(HaveOccurred(), "Failed to get %q pod", cpuConsumerPodName)
		Expect(ms.Containers).To(HaveLen(1), "Unexpected number of containers")
		usage := ms.Containers[0].Usage
		Expect(usage.Cpu().MilliValue()).To(BeNumerically("~", 50, 10), "Unexpected value of cpu")
	})
	It("returns accurate memory metric", func() {
		Expect(err).NotTo(HaveOccurred(), "Failed to create %q pod", memoryConsumerPodName)
		deadline := time.Now().Add(60 * time.Second)
		var ms *v1beta1.PodMetrics
		for {
			ms, err = mclient.MetricsV1beta1().PodMetricses(metav1.NamespaceDefault).Get(context.TODO(), memoryConsumerPodName, metav1.GetOptions{})
			if err == nil || time.Now().After(deadline) {
				break
			}
			time.Sleep(5 * time.Second)
		}
		Expect(err).NotTo(HaveOccurred(), "Failed to get %q pod", memoryConsumerPodName)
		Expect(ms.Containers).To(HaveLen(1), "Unexpected number of containers")
		usage := ms.Containers[0].Usage
		Expect(usage.Memory().Value()/1024/1024).To(BeNumerically("~", 50, 5), "Unexpected value of memory")
	})
	It("returns metric for pod with init container", func() {
		Expect(err).NotTo(HaveOccurred(), "Failed to create %q pod", initContainerPodName)
		deadline := time.Now().Add(60 * time.Second)
		var ms *v1beta1.PodMetrics
		for {
			ms, err = mclient.MetricsV1beta1().PodMetricses(metav1.NamespaceDefault).Get(context.TODO(), initContainerPodName, metav1.GetOptions{})
			if err == nil || time.Now().After(deadline) {
				break
			}
			time.Sleep(5 * time.Second)
		}
		Expect(err).NotTo(HaveOccurred(), "Failed to get %q pod", initContainerPodName)
		Expect(ms.Containers).To(HaveLen(1), "Unexpected number of containers")
		Expect(ms.Containers[0].Name).To(Equal(initContainerPodName))
		usage := ms.Containers[0].Usage
		Expect(usage.Cpu().MilliValue()).NotTo(Equal(0), "CPU should not be equal zero")
		Expect(usage.Memory().Value()/1024/1024).NotTo(Equal(0), "Memory should not be equal zero")
	})
	It("returns metric for pod with sideCar container", func() {
		Expect(err).NotTo(HaveOccurred(), "Failed to create %q pod", sideCarContainerPodName)
		deadline := time.Now().Add(60 * time.Second)
		var ms *v1beta1.PodMetrics
		for {
			ms, err = mclient.MetricsV1beta1().PodMetricses(metav1.NamespaceDefault).Get(context.TODO(), sideCarContainerPodName, metav1.GetOptions{})
			if err == nil || time.Now().After(deadline) {
				break
			}
			time.Sleep(5 * time.Second)
		}
		Expect(err).NotTo(HaveOccurred(), "Failed to get %q pod", sideCarContainerPodName)
		Expect(ms.Containers).To(HaveLen(2), "Unexpected number of containers")
		usage := ms.Containers[0].Usage
		Expect(usage.Cpu().MilliValue()).NotTo(Equal(0), "CPU of Container %q should not be equal zero", ms.Containers[0].Name)
		Expect(usage.Memory().Value()/1024/1024).NotTo(Equal(0), "Memory of Container %q should not be equal zero", ms.Containers[0].Name)
		usage = ms.Containers[1].Usage
		Expect(usage.Cpu().MilliValue()).NotTo(Equal(0), "CPU of Container %q should not be equal zero", ms.Containers[1].Name)
		Expect(usage.Memory().Value()/1024/1024).NotTo(Equal(0), "Memory of Container %q should not be equal zero", ms.Containers[1].Name)
	})

	if testSideCarsContainers {
		It("returns metric for pod with init sidecar container", func() {
			Expect(err).NotTo(HaveOccurred(), "Failed to create %q pod", initSidecarContainersPodName)
			deadline := time.Now().Add(60 * time.Second)
			var ms *v1beta1.PodMetrics
			for {
				ms, err = mclient.MetricsV1beta1().PodMetricses(metav1.NamespaceDefault).Get(context.TODO(), initSidecarContainersPodName, metav1.GetOptions{})
				if err == nil || time.Now().After(deadline) {
					break
				}
				time.Sleep(5 * time.Second)
			}
			Expect(err).NotTo(HaveOccurred(), "Failed to get %q pod", initSidecarContainersPodName)
			Expect(ms.Containers).To(HaveLen(2), "Unexpected number of containers")
			usage := ms.Containers[0].Usage
			Expect(usage.Cpu().MilliValue()).NotTo(Equal(0), "CPU should not be equal zero")
			Expect(usage.Memory().Value()/1024/1024).NotTo(Equal(0), "Memory should not be equal zero")
			usage = ms.Containers[1].Usage
			Expect(usage.Cpu().MilliValue()).NotTo(Equal(0), "CPU should not be equal zero")
			Expect(usage.Memory().Value()/1024/1024).NotTo(Equal(0), "Memory should not be equal zero")
		})
	}
	It("passes readyz probe", func() {
		msPods := mustGetMetricsServerPods(client)
		for _, pod := range msPods {
			Expect(pod.Spec.Containers).To(HaveLen(1), "Expected only one container in Metrics Server pod")
			resp := mustProxyContainerProbe(restConfig, pod.Namespace, pod.Name, pod.Spec.Containers[0], pod.Spec.Containers[0].ReadinessProbe)
			diff := cmp.Diff(string(resp), `[+]ping ok
[+]log ok
[+]poststarthook/max-in-flight-filter ok
[+]poststarthook/storage-object-count-tracker-hook ok
[+]metric-storage-ready ok
[+]metric-informer-sync ok
[+]metadata-informer-sync ok
[+]shutdown ok
readyz check passed
`)
			Expect(diff == "").To(BeTrue(), "Unexpected response %s", diff)
		}
	})
	It("passes livez probe", func() {
		msPods := mustGetMetricsServerPods(client)
		for _, pod := range msPods {
			Expect(pod.Spec.Containers).To(HaveLen(1), "Expected only one container in Metrics Server pod")
			resp := mustProxyContainerProbe(restConfig, pod.Namespace, pod.Name, pod.Spec.Containers[0], pod.Spec.Containers[0].LivenessProbe)
			diff := cmp.Diff(string(resp), `[+]ping ok
[+]log ok
[+]poststarthook/max-in-flight-filter ok
[+]poststarthook/storage-object-count-tracker-hook ok
[+]metric-collection-timely ok
[+]metadata-informer-sync ok
livez check passed
`)
			Expect(diff == "").To(BeTrue(), "Unexpected response %s", diff)
		}
	})
	It("exposes prometheus metrics", func() {
		msPods := mustGetMetricsServerPods(client)
		for _, pod := range msPods {
			resp, err := proxyRequestToPod(restConfig, pod.Namespace, pod.Name, "https", 10250, "/metrics")
			Expect(err).NotTo(HaveOccurred(), "Failed to get Metrics Server /metrics endpoint")
			metrics, err := parseMetricNames(resp)
			Expect(err).NotTo(HaveOccurred(), "Failed to parse Metrics Server metrics")
			sort.Strings(metrics)

			diff := cmp.Diff(metrics, []string{
				"metrics_server_api_metric_freshness_seconds",
				"metrics_server_kubelet_last_request_time_seconds",
				"metrics_server_kubelet_request_duration_seconds",
				"metrics_server_kubelet_request_total",
				"metrics_server_manager_tick_duration_seconds",
				"metrics_server_storage_points",
			})
			Expect(diff).To(BeEmpty(), "Unexpected metrics")
		}
	})
	It("skip scrape metrics about nodes with label node-selector filtered in cluster", func() {
		nodeList, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: skipLabel})
		if err != nil {
			panic(err)
		}
		for _, node := range nodeList.Items {
			_, err := mclient.MetricsV1beta1().NodeMetricses().Get(context.TODO(), node.Name, metav1.GetOptions{})
			Expect(err).To(HaveOccurred(), "Metrics for node %s are not available with label node-selector filtered", node.Name)
		}
	})
})

func getRestConfig() (*rest.Config, error) {
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return nil, err
	}
	return clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
}

func mustGetMetricsServerPods(client clientset.Interface) []corev1.Pod {
	podList, err := client.CoreV1().Pods(metav1.NamespaceSystem).List(context.TODO(), metav1.ListOptions{LabelSelector: "k8s-app=metrics-server"})
	Expect(err).NotTo(HaveOccurred(), "Failed to find Metrics Server pod")
	Expect(podList.Items).NotTo(BeEmpty(), "Metrics Server pod was not found")
	return podList.Items
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
		if !strings.HasPrefix(key, "metrics_server_") {
			continue
		}
		ms = append(ms, key)
	}
	return ms, nil
}

func mustProxyContainerProbe(config *rest.Config, namespace, name string, container corev1.Container, probe *corev1.Probe) []byte {
	Expect(probe).NotTo(BeNil(), "Probe should not be empty")
	Expect(probe.HTTPGet).NotTo(BeNil(), "Probe should be http")
	port := getContainerPort(container, probe.HTTPGet.Port)
	Expect(port).NotTo(Equal(0), "Probe port should not be empty")
	resp, err := proxyRequestToPod(config, namespace, name, string(probe.HTTPGet.Scheme), port, mustGetVerboseProbePath(probe.HTTPGet))
	Expect(err).NotTo(HaveOccurred(), "Failed to get Metrics Server probe endpoint")
	return resp
}

func mustGetVerboseProbePath(probe *corev1.HTTPGetAction) string {
	path, err := url.Parse(probe.Path)
	Expect(err).NotTo(HaveOccurred(), "Failed to parse probe path")
	newPath := url.URL{
		Path:     path.Path,
		RawQuery: "verbose&" + path.RawQuery,
	}
	return newPath.String()
}

func getContainerPort(c corev1.Container, port intstr.IntOrString) int {
	switch port.Type {
	case intstr.Int:
		return int(port.IntVal)
	case intstr.String:
		for _, cp := range c.Ports {
			if cp.Name == port.StrVal {
				return int(cp.ContainerPort)
			}
		}
	}
	return 0
}

func proxyRequestToPod(config *rest.Config, namespace, podname, scheme string, port int, path string) ([]byte, error) {
	cancel, err := setupForwarding(config, namespace, podname, port)
	defer cancel()
	if err != nil {
		return nil, err
	}
	var query string
	if strings.Contains(path, "?") {
		elm := strings.SplitN(path, "?", 2)
		path = elm[0]
		query = elm[1]
	}
	reqURL := url.URL{Scheme: scheme, Path: path, RawQuery: query, Host: fmt.Sprintf("127.0.0.1:%d", localPort)}
	resp, err := sendRequest(config, reqURL.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func setupForwarding(config *rest.Config, namespace, podname string, port int) (cancel func(), err error) {
	hostIP := strings.TrimPrefix(config.Host, "https://")
	mappings := []string{fmt.Sprintf("%d:%d", localPort, port)}

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

	fw, err := portforward.New(dialer, mappings, stopCh, readyCh, buffOut, buffErr)
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
	tsConfig.TLS.CAFile = ""

	ts, err := transport.New(tsConfig)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Transport: ts}
	return client.Get(url)
}

func noop() {}

func watchPodReadyStatus(client clientset.Interface, podNamespace string, podName string, resourceVersion string) error {
	timeout := time.After(time.Second * 300)
	var api = client.CoreV1().Pods(podNamespace)
	watcher, err := api.Watch(context.TODO(), metav1.ListOptions{ResourceVersion: resourceVersion})
	if err != nil {
		return err
	}
	ch := watcher.ResultChan()
	for {
		select {
		case <-timeout:
			return fmt.Errorf("Wait for pod %s ready timeout", podName)
		case event := <-ch:
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				return fmt.Errorf("Watch pod failed")
			}
			if pod.Name == podName {
				if checkPodContainersReady(pod) {
					return nil
				}
			}
		}
	}
}

func consumeCPU(client clientset.Interface, podName, nodeSelector string) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    podName,
					Command: []string{"./consume-cpu/consume-cpu"},
					Args:    []string{"--duration-sec=600", "--millicores=50"},
					Image:   "registry.k8s.io/e2e-test-images/resource-consumer:1.9",
					Resources: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU: mustQuantity("100m"),
						},
					},
				},
			},
			Affinity: affinity(nodeSelector),
		},
	}

	currentPod, err := client.CoreV1().Pods(metav1.NamespaceDefault).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return watchPodReadyStatus(client, metav1.NamespaceDefault, podName, currentPod.ResourceVersion)
}

func consumeMemory(client clientset.Interface, podName, nodeSelector string) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    podName,
					Command: []string{"stress"},
					Args:    []string{"-m", "1", "--vm-bytes", "50M", "--vm-hang", "0", "-t", "600"},
					Image:   "registry.k8s.io/e2e-test-images/resource-consumer:1.9",
					Resources: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceMemory: mustQuantity("100Mi"),
						},
					},
				},
			},
			Affinity: affinity(nodeSelector),
		},
	}
	currentPod, err := client.CoreV1().Pods(metav1.NamespaceDefault).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return watchPodReadyStatus(client, metav1.NamespaceDefault, podName, currentPod.ResourceVersion)
}

func consumeWithInitContainer(client clientset.Interface, podName, nodeSelector string) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    podName,
					Command: []string{"./consume-cpu/consume-cpu"},
					Args:    []string{"--duration-sec=600", "--millicores=50"},
					Image:   "registry.k8s.io/e2e-test-images/resource-consumer:1.9",
					Resources: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:    mustQuantity("100m"),
							corev1.ResourceMemory: mustQuantity("100Mi"),
						},
					},
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:    "init-container",
					Command: []string{"./consume-cpu/consume-cpu"},
					Args:    []string{"--duration-sec=10", "--millicores=50"},
					Image:   "registry.k8s.io/e2e-test-images/resource-consumer:1.9",
				},
			},
			Affinity: affinity(nodeSelector),
		},
	}

	currentPod, err := client.CoreV1().Pods(metav1.NamespaceDefault).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return watchPodReadyStatus(client, metav1.NamespaceDefault, podName, currentPod.ResourceVersion)
}

func consumeWithSideCarContainer(client clientset.Interface, podName, nodeSelector string) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    podName,
					Command: []string{"./consume-cpu/consume-cpu"},
					Args:    []string{"--duration-sec=600", "--millicores=50"},
					Image:   "registry.k8s.io/e2e-test-images/resource-consumer:1.9",
					Resources: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:    mustQuantity("100m"),
							corev1.ResourceMemory: mustQuantity("100Mi"),
						},
					},
				},
				{
					Name:    "sidecar-container",
					Command: []string{"./consume-cpu/consume-cpu"},
					Args:    []string{"--duration-sec=600", "--millicores=50"},
					Image:   "registry.k8s.io/e2e-test-images/resource-consumer:1.9",
					Resources: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:    mustQuantity("100m"),
							corev1.ResourceMemory: mustQuantity("100Mi"),
						},
					},
				},
			},
			Affinity: affinity(nodeSelector),
		},
	}

	currentPod, err := client.CoreV1().Pods(metav1.NamespaceDefault).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return watchPodReadyStatus(client, metav1.NamespaceDefault, podName, currentPod.ResourceVersion)
}

func consumeWithInitSideCarContainer(client clientset.Interface, podName, nodeSelector string) error {
	startPolicy := corev1.ContainerRestartPolicyAlways
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    podName,
					Command: []string{"./consume-cpu/consume-cpu"},
					Args:    []string{"--duration-sec=600", "--millicores=50"},
					Image:   "registry.k8s.io/e2e-test-images/resource-consumer:1.9",
					Resources: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:    mustQuantity("100m"),
							corev1.ResourceMemory: mustQuantity("100Mi"),
						},
					},
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:          "init-container",
					Command:       []string{"./consume-cpu/consume-cpu"},
					Args:          []string{"--duration-sec=600", "--millicores=50"},
					Image:         "registry.k8s.io/e2e-test-images/resource-consumer:1.9",
					RestartPolicy: &startPolicy,
					Resources: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:    mustQuantity("100m"),
							corev1.ResourceMemory: mustQuantity("100Mi"),
						},
					},
				},
			},
			Affinity: affinity(nodeSelector),
		},
	}

	currentPod, err := client.CoreV1().Pods(metav1.NamespaceDefault).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return watchPodReadyStatus(client, metav1.NamespaceDefault, podName, currentPod.ResourceVersion)
}

func deletePod(client clientset.Interface, podName string) {
	var gracePeriodSeconds int64 = 0
	_ = client.CoreV1().Pods(metav1.NamespaceDefault).Delete(context.TODO(), podName, metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriodSeconds,
	})
}

func mustQuantity(s string) resource.Quantity {
	q, err := resource.ParseQuantity(s)
	if err != nil {
		panic(err)
	}
	return q
}
func affinity(key string) *corev1.Affinity {
	return &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      key,
								Operator: corev1.NodeSelectorOpDoesNotExist,
							},
						},
					},
				},
			},
		},
	}
}

func checkPodContainersReady(pod *corev1.Pod) bool {
	for _, containerStatus := range pod.Status.InitContainerStatuses {
		if !containerStatus.Ready {
			return false
		}
	}
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if !containerStatus.Ready {
			return false
		}
	}
	return true
}

func hasSidecarFeatureEnabled(client clientset.Interface) bool {
	if apiServerPod, err := client.CoreV1().Pods("kube-system").Get(context.TODO(), "kube-apiserver-e2e-control-plane", metav1.GetOptions{}); err == nil {
		cmds := apiServerPod.Spec.Containers[0].Command
		for index := range cmds {
			if strings.Contains(cmds[index], "--feature-gates") && strings.Contains(cmds[index], "SidecarContainers=true") {
				return true
			}
		}
	}
	return false
}
