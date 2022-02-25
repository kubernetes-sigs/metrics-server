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
	"bytes"
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/metrics/pkg/apis/metrics"
	"sigs.k8s.io/metrics-server/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/metrics-server/pkg/scraper"
	"sigs.k8s.io/metrics-server/pkg/storage"

	sclient "sigs.k8s.io/metrics-server/pkg/scraper/client"
)

type fakeKubeletClient struct {
	defaultPort       int
	useNodeStatusPort bool
	client            *http.Client
	scheme            string
	addrResolver      utils.NodeAddressResolver
	buffers           sync.Pool
}

var _ sclient.KubeletMetricsGetter = (*fakeKubeletClient)(nil)

func (c *fakeKubeletClient) GetMetrics(ctx context.Context, node *corev1.Node) (*storage.MetricsBatch, error) {
	return &storage.MetricsBatch{
		Nodes: make(map[string]storage.MetricsPoint),
		Pods:  make(map[apitypes.NamespacedName]storage.PodMetricsPoint),
	}, nil
}

func TestServer(t *testing.T) {
	// Filter goroutine leak detection for klog
	o := goleak.IgnoreCurrent()
	defer goleak.VerifyNone(t, o)
	resolution := 10 * time.Second
	client := fake.NewSimpleClientset(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1", UID: "0011-2233-1"}}, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node2", UID: "0011-2233-2"}},
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node3", UID: "0011-2233-3"}}, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node4", UID: "0011-2233-4"}}, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node5", UID: "0011-2233-5"}},
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node6", UID: "0011-2233-6"}}, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node7", UID: "0011-2233-7"}}, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node8", UID: "0011-2233-8"}},
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node9", UID: "0011-2233-9"}}, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node10", UID: "0011-2233-10"}})
	kubeconfig := &rest.Config{
		Host:            "https://10.96.0.1:443",
		APIPath:         "",
		Username:        "Username",
		Password:        "Password",
		BearerToken:     "ApiserverBearerToken",
		BearerTokenFile: "ApiserverBearerTokenFile",
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: false,
			CertFile: "CertFile",
			KeyFile:  "KeyFile",
			CAFile:   "CAFile",
			CertData: []byte("CertData"),
			KeyData:  []byte("KeyData"),
			CAData:   []byte("CAData"),
		},
		UserAgent: "UserAgent",
	}
	transport, _ := rest.TransportFor(kubeconfig)
	kubeletClient := &fakeKubeletClient{
		defaultPort:       10250,
		useNodeStatusPort: true,
		client: &http.Client{
			Transport: transport,
		},
		scheme:       "https",
		addrResolver: utils.NewPriorityNodeAddressResolver([]corev1.NodeAddressType{"Hostname", "InternalDNS", "InternalIP", "ExternalDNS", "ExternalIP"}),
		buffers: sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}
	nodeInformer := informers.NewSharedInformerFactory(client, 0).Core().V1().Nodes().Informer()
	podInformer := informers.NewSharedInformerFactory(client, 0).Core().V1().Pods().Informer()
	store := &storageMock{}
	manageNodeScrape := scraper.NewScraper(kubeletClient, 2*time.Second, 5*time.Second, store)
	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(node interface{}) {
			if err := manageNodeScrape.AddNodeScraper(node.(*corev1.Node)); err != nil {
				t.Error(err)
			}
		},
		DeleteFunc: func(node interface{}) {
			manageNodeScrape.DeleteNodeScraper(node.(*corev1.Node))
		},
	})

	server := NewServer(nodeInformer, podInformer, nil, store, resolution)
	stopCh := make(chan struct{})
	defer close(stopCh)
	go server.nodes.Run(stopCh)
	go server.pods.Run(stopCh)
	time.Sleep(10 * time.Second)
	manageNodeScrape.DeleteNodeScraper(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1", UID: "0011-2233-1"}})
	manageNodeScrape.DeleteNodeScraper(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node2", UID: "0011-2233-2"}})
	manageNodeScrape.DeleteNodeScraper(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node3", UID: "0011-2233-3"}})
	manageNodeScrape.DeleteNodeScraper(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node4", UID: "0011-2233-4"}})
	manageNodeScrape.DeleteNodeScraper(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node5", UID: "0011-2233-5"}})
	manageNodeScrape.DeleteNodeScraper(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node6", UID: "0011-2233-6"}})
	manageNodeScrape.DeleteNodeScraper(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node7", UID: "0011-2233-7"}})
	manageNodeScrape.DeleteNodeScraper(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node8", UID: "0011-2233-8"}})
	manageNodeScrape.DeleteNodeScraper(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node9", UID: "0011-2233-9"}})
	manageNodeScrape.DeleteNodeScraper(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node10", UID: "0011-2233-10"}})
	time.Sleep(10 * time.Second)
	stopCh <- struct{}{}
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
func (s *storageMock) DiscardNode(node corev1.Node) {
}
func (s *storageMock) DiscardPods(podsRef []apitypes.NamespacedName) {
}
