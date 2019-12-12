package test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

func TestMetricsServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "[MetricsServer]")
}

var _ = Describe("[MetricsServer]", func() {
	client, err := getKubernetesClient()
	if err != nil {
		panic(err)
	}
	mclient, err := getMetricsClient()
	if err != nil {
		panic(err)
	}
	It("Metrics for each pod are available", func() {
		podList, err := client.CoreV1().Pods("").List(metav1.ListOptions{})
		if err != nil {
			panic(err)
		}
		Expect(podList.Items).NotTo(BeEmpty(), "Need at least one pod to verify if MetricsServer works")
		for _, pod := range podList.Items {
			_, err := mclient.MetricsV1beta1().PodMetricses(pod.Namespace).Get(pod.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "Metrics for pod %s/%s are not available", pod.Namespace, pod.Name)
		}
	})
	It("Metrics for each node are available", func() {
		nodeList, err := client.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			panic(err)
		}
		Expect(nodeList.Items).NotTo(BeEmpty(), "Need at least one node to verify if MetricsServer works")
		for _, node := range nodeList.Items {
			_, err := mclient.MetricsV1beta1().NodeMetricses().Get(node.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "Metrics for node %s are not available", node.Name)
		}
	})
})

func getKubernetesClient() (clientset.Interface, error) {
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return nil, err
	}
	clientConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, err
	}
	return clientset.NewForConfig(clientConfig)
}

func getMetricsClient() (metricsclientset.Interface, error) {
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return nil, err
	}
	clientConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, err
	}
	return metricsclientset.NewForConfig(clientConfig)
}
