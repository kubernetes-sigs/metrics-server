// Copyright 2022 The Kubernetes Authors.
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

package server

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	apitypes "k8s.io/apimachinery/pkg/types"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/metrics-server/pkg/scraper"
	"sigs.k8s.io/metrics-server/pkg/scraper/client"
	"sigs.k8s.io/metrics-server/pkg/storage"
)

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

type fakeClient struct {
	metrics map[string]*storage.MetricsBatch
}
type fakeLister struct {
	nodes   []*corev1.Node
	listErr error
}
type scrapeMock struct {
	result *storage.MetricsBatch
}

var _ client.KubeletMetricsGetter = (*fakeClient)(nil)
var _ scraper.Scraper = (*scrapeMock)(nil)

type scenario struct {
	name            string
	nodeCount       int
	podsPerNode     int
	deploymentCount int
	namespaceCount  int
	containerPerPod int
}

var scenarios = []scenario{
	{
		name:            "Normal 1000",
		nodeCount:       1000,
		podsPerNode:     30,
		deploymentCount: 100,
		namespaceCount:  10,
		containerPerPod: 2,
	},
	{
		name:            "Normal 100",
		nodeCount:       100,
		podsPerNode:     70,
		deploymentCount: 100,
		namespaceCount:  10,
		containerPerPod: 2,
	},
	{
		name:            "Normal 10",
		nodeCount:       10,
		podsPerNode:     70,
		deploymentCount: 100,
		namespaceCount:  10,
		containerPerPod: 2,
	},
	{
		name:            "Big Namespace 1000",
		nodeCount:       1000,
		podsPerNode:     30,
		deploymentCount: 100,
		namespaceCount:  1,
		containerPerPod: 2,
	},
	{
		name:            "Big Namespace 100",
		nodeCount:       100,
		podsPerNode:     70,
		deploymentCount: 100,
		namespaceCount:  1,
		containerPerPod: 2,
	},
	{
		name:            "Big Namespace 10",
		nodeCount:       10,
		podsPerNode:     70,
		deploymentCount: 100,
		namespaceCount:  1,
		containerPerPod: 2,
	},
	{
		name:            "Big Deployment 1000",
		nodeCount:       1000,
		podsPerNode:     30,
		deploymentCount: 1,
		namespaceCount:  1,
		containerPerPod: 2,
	},
	{
		name:            "Big Deployment 100",
		nodeCount:       100,
		podsPerNode:     70,
		deploymentCount: 1,
		namespaceCount:  1,
		containerPerPod: 2,
	},
	{
		name:            "Big Deployment 10",
		nodeCount:       10,
		podsPerNode:     70,
		deploymentCount: 1,
		namespaceCount:  1,
		containerPerPod: 2,
	},
	{
		name:            "Dense Container 100",
		nodeCount:       100,
		podsPerNode:     30,
		deploymentCount: 100,
		namespaceCount:  10,
		containerPerPod: 10,
	},
	{
		name:            "Dense Container 10",
		nodeCount:       10,
		podsPerNode:     30,
		deploymentCount: 100,
		namespaceCount:  10,
		containerPerPod: 10,
	},
}

func BenchmarkScraper(b *testing.B) {
	for _, s := range scenarios {
		r := rand.New(rand.NewSource(1))
		g := newGenerator(r, s)
		b.Run(s.name, func(b *testing.B) {
			benchmarkScraper(b, g)
		})
	}
}

func benchmarkScraper(b *testing.B, g *generator) {
	nodes := fakeLister{nodes: g.nodes}
	scraper := scraper.NewScraper(&nodes, &g.client, g.scrapeTimeout)
	server := NewServer(g.nodeInformer, g.podInformer, g.apiserver, g.store, scraper, g.metricResolution)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopCh := make(chan struct{})
	defer close(stopCh)
	go server.nodes.Run(stopCh)
	go server.pods.Run(stopCh)
	go server.runScrape(ctx)
	b.ResetTimer()
	b.ReportAllocs()
	time.Sleep(16 * time.Second)
	stopCh <- struct{}{}
}

type generator struct {
	containerPerPod  int
	nodes            []*corev1.Node
	nodesObject      []runtime.Object
	nodePods         map[string][]string
	deploymentPods   map[string][]string
	podNamespace     map[string]string
	rand             *rand.Rand
	scrapeTimeout    time.Duration
	metricResolution time.Duration
	nodeInformer     cache.Controller
	podInformer      cache.Controller
	client           fakeClient
	store            *storageMock
	nodeUID          uint64

	apiserver *genericapiserver.GenericAPIServer

	cumulativeCpuUsed uint64
}

func newGenerator(rand *rand.Rand, s scenario) *generator {
	g := generator{
		rand:              rand,
		containerPerPod:   s.containerPerPod,
		cumulativeCpuUsed: 0,
		scrapeTimeout:     3 * time.Second,
		metricResolution:  5 * time.Second,
	}
	podCount := s.podsPerNode * s.nodeCount
	podsPerDeployment := podCount / s.deploymentCount
	podsRest := podCount % s.deploymentCount

	namespaceNames := []string{}
	for i := 0; i < s.namespaceCount; i++ {
		name := g.RandomString(20)
		namespaceNames = append(namespaceNames, name)
	}

	nodePods := map[string][]string{}
	nodeNames := []string{}
	for i := 0; i < s.nodeCount; i++ {
		name := fmt.Sprintf("node-%s", g.RandomString(20))
		nodePods[name] = []string{}
		nodeNames = append(nodeNames, name)
		g.nodes = append(g.nodes, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}})
		g.nodeUID += 1
		g.nodesObject = append(g.nodesObject, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID(fmt.Sprintf("%v", g.nodeUID))}})
	}
	deploymentPods := map[string][]string{}
	deploymentNamespace := map[string]string{}
	podNamespace := map[string]string{}
	for i := 0; i < s.deploymentCount; i++ {
		deploy := g.RandomString(10)
		namespace := namespaceNames[g.rand.Intn(len(namespaceNames))]
		deploymentNamespace[deploy] = namespace
		pods := []string{}
		podsCount := podsPerDeployment
		if i < podsRest {
			podsCount += 1
		}
		for j := 0; j < podsCount; j++ {
			pod := deploy + "-" + g.RandomString(10)
			pods = append(pods, pod)
			node := nodeNames[g.rand.Intn(len(nodeNames))]
			nodePods[node] = append(nodePods[node], pod)
			podNamespace[pod] = namespace
		}
		deploymentPods[deploy] = pods
	}
	g.nodePods = nodePods
	g.deploymentPods = deploymentPods
	g.podNamespace = podNamespace
	g.store = &storageMock{}
	client := fake.NewSimpleClientset(g.nodesObject...)
	g.client = fakeClient{
		metrics: *g.NewBatch(),
	}
	g.nodeInformer = informers.NewSharedInformerFactory(client, 0).Core().V1().Nodes().Informer()
	g.podInformer = informers.NewSharedInformerFactory(client, 0).Core().V1().Pods().Informer()
	return &g
}
func (g *generator) NewBatch() *map[string]*storage.MetricsBatch {
	mbMap := make(map[string]*storage.MetricsBatch)
	containerNames := []string{}
	for i := 0; i < g.containerPerPod; i++ {
		containerNames = append(containerNames, fmt.Sprintf("container-%d", i))
	}
	for _, node := range g.nodes {
		mb := storage.MetricsBatch{
			Nodes: map[string]storage.MetricsPoint{},
			Pods:  map[apitypes.NamespacedName]storage.PodMetricsPoint{},
		}
		nodePods := g.nodePods[node.Name]
		for _, pod := range nodePods {
			point := storage.PodMetricsPoint{
				Containers: map[string]storage.MetricsPoint{},
			}
			for i := 0; i < g.containerPerPod; i++ {
				point.Containers[containerNames[i]] = g.RandomMetricsPoint()
			}
			mb.Pods[apitypes.NamespacedName{Name: pod, Namespace: g.podNamespace[pod]}] = point
		}
		mb.Nodes[node.Name] = g.RandomMetricsPoint()
		mbMap[node.Name] = &mb
	}
	return &mbMap
}
func (g *generator) RandomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[g.rand.Intn(len(charset))]
	}
	return string(b)
}

func (g *generator) RandomMetricsPoint() storage.MetricsPoint {
	g.cumulativeCpuUsed += uint64(g.rand.Int63n(1e8))
	return storage.MetricsPoint{
		Timestamp:         time.Now(),
		CumulativeCpuUsed: g.cumulativeCpuUsed,
		MemoryUsage:       g.rand.Uint64(),
	}
}
func (c *fakeClient) GetMetrics(ctx context.Context, node *corev1.Node) (*storage.MetricsBatch, error) {
	metrics, ok := c.metrics[node.Name]
	if !ok {
		return nil, fmt.Errorf("Unknown node %q", node.Name)
	}
	return metrics, nil
}
func (s *scrapeMock) Scrape(ctx context.Context) *storage.MetricsBatch {
	return s.result
}

func (l *fakeLister) List(_ labels.Selector) (ret []*corev1.Node, err error) {
	if l.listErr != nil {
		return nil, l.listErr
	}
	// NB: this is ignores selector for the moment
	return l.nodes, nil
}

func (l *fakeLister) Get(name string) (*corev1.Node, error) {
	for _, node := range l.nodes {
		if node.Name == name {
			return node, nil
		}
	}
	return nil, fmt.Errorf("no such node %q", name)
}
