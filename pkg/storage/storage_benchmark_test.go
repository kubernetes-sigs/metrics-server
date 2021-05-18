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

package storage

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apitypes "k8s.io/apimachinery/pkg/types"
)

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

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

func BenchmarkStorageWrite(b *testing.B) {
	for _, s := range scenarios {
		r := rand.New(rand.NewSource(1))
		g := newGenerator(r, s)
		b.Run(s.name, func(b *testing.B) {
			benchmarkStorageWrite(b, g)
		})
	}
}

func benchmarkStorageWrite(b *testing.B, g *generator) {
	s := NewStorage(60 * time.Second)
	// Limit size to limit memory needed
	maxSize := 100
	if maxSize > b.N {
		maxSize = b.N
	}
	bs := make([]*MetricsBatch, 0, maxSize)
	for i := 0; i < maxSize; i++ {
		bs = append(bs, g.NewBatch())
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s.Store(bs[i%maxSize])
	}
}

func BenchmarkStorageReadContainer(b *testing.B) {
	for _, s := range scenarios {
		r := rand.New(rand.NewSource(1))
		g := newGenerator(r, s)
		b.Run(s.name, func(b *testing.B) {
			benchmarkStorageReadContainer(b, g)
		})
	}
}

func benchmarkStorageReadContainer(b *testing.B, g *generator) {
	s := NewStorage(60 * time.Second)
	s.Store(g.NewBatch())
	s.Store(g.NewBatch())
	deployments := g.Deployments()
	queries := [][]apitypes.NamespacedName{}
	for _, d := range deployments {
		queries = append(queries, g.NamespacesNames(d))
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, q := range queries {
			ts, cs, err := s.GetPodMetrics(q...)
			if err != nil {
				panic(err)
			}
			for i := range q {
				if ts[i].Timestamp.IsZero() {
					panic(fmt.Sprintf("%s: Expect to get all timeseries, expected: %d, got: %d", b.Name(), len(q), i+1))
				}
				if len(cs[i]) == 0 {
					panic(fmt.Sprintf("%s: Expect to get all metrics, expected: %d, got: %d", b.Name(), len(q), i+1))
				}
			}
		}
	}
}

func BenchmarkStorageReadNode(b *testing.B) {
	for _, s := range scenarios {
		r := rand.New(rand.NewSource(1))
		g := newGenerator(r, s)
		b.Run(s.name, func(b *testing.B) {
			benchmarkStorageReadNode(b, g)
		})
	}
}

func benchmarkStorageReadNode(b *testing.B, g *generator) {
	s := NewStorage(60 * time.Second)
	s.Store(g.NewBatch())
	s.Store(g.NewBatch())
	nodes := g.Nodes()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ts, ns, err := s.GetNodeMetrics(nodes...)
		if err != nil {
			panic(err)
		}
		for i := range nodes {
			if ts[i].Timestamp.IsZero() {
				panic(fmt.Sprintf("%s: Expect to get all timeseries, expected: %d, got: %d", b.Name(), len(nodes), i+1))
			}
			if len(ns[i]) == 0 {
				panic(fmt.Sprintf("%s: Expect to get all metrics, expected: %d, got: %d", b.Name(), len(nodes), i+1))
			}
		}
	}
}

type generator struct {
	containerPerPod int
	nodePods        map[string][]string
	deploymentPods  map[string][]string
	podNamespace    map[string]string
	rand            *rand.Rand
}

func newGenerator(rand *rand.Rand, s scenario) *generator {
	g := generator{
		rand:            rand,
		containerPerPod: s.containerPerPod,
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
	return &g
}

func (g *generator) NewBatch() *MetricsBatch {
	mb := MetricsBatch{
		Nodes: []NodeMetricsPoint{},
		Pods:  []PodMetricsPoint{},
	}
	containerNames := []string{}
	for i := 0; i < g.containerPerPod; i++ {
		containerNames = append(containerNames, fmt.Sprintf("container-%d", i))
	}

	for node, pods := range g.nodePods {
		for _, pod := range pods {
			point := PodMetricsPoint{
				Name:       pod,
				Namespace:  g.podNamespace[pod],
				Containers: []ContainerMetricsPoint{},
			}
			for i := 0; i < g.containerPerPod; i++ {
				cont := ContainerMetricsPoint{
					Name:         containerNames[i],
					MetricsPoint: g.RandomMetricsPoint(),
				}
				point.Containers = append(point.Containers, cont)
			}
			mb.Pods = append(mb.Pods, point)
		}
		mb.Nodes = append(mb.Nodes, NodeMetricsPoint{
			Name:         node,
			MetricsPoint: g.RandomMetricsPoint(),
		})
	}
	return &mb
}

func (g *generator) Nodes() []string {
	nodes := []string{}
	for d := range g.nodePods {
		nodes = append(nodes, d)
	}
	return nodes
}

func (g *generator) Deployments() []string {
	deployments := []string{}
	for d := range g.deploymentPods {
		deployments = append(deployments, d)
	}
	return deployments
}

func (g *generator) NamespacesNames(deployment string) []apitypes.NamespacedName {
	namespacesNames := []apitypes.NamespacedName{}
	for _, pod := range g.deploymentPods[deployment] {
		namespacesNames = append(namespacesNames, apitypes.NamespacedName{
			Namespace: g.podNamespace[pod],
			Name:      pod,
		})
	}
	return namespacesNames

}

func (g *generator) RandomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[g.rand.Intn(len(charset))]
	}
	return string(b)
}

func (g *generator) RandomMetricsPoint() MetricsPoint {
	return MetricsPoint{
		Timestamp:         time.Now(),
		CumulativeCpuUsed: g.rand.Uint64(),
		MemoryUsage:       g.rand.Uint64(),
	}
}

func TestStorageBenchmarkGenerator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "storage benchmark generator Suite")
}

var _ = Describe("Test generator", func() {
	It("should generate correct output", func() {
		r := rand.New(rand.NewSource(1))
		s := scenario{
			name:            "Test",
			nodeCount:       1,
			namespaceCount:  2,
			podsPerNode:     5,
			deploymentCount: 3,
			containerPerPod: 7,
		}

		g := newGenerator(r, s)
		nodes := g.Nodes()
		sort.Strings(nodes)
		Expect(nodes).To(HaveLen(s.nodeCount), "Nodes count not match")
		deployments := g.Deployments()
		sort.Strings(deployments)
		Expect(deployments).To(HaveLen(s.deploymentCount), "Deployments count not match")
		stats := g.NewBatch()
		Expect(stats.Nodes).To(HaveLen(s.nodeCount), "Node metric count not match")
		Expect(stats.Pods).To(HaveLen(s.nodeCount*s.podsPerNode), "Pod metric count not match")
		ns := map[string]struct{}{}
		for _, pod := range stats.Pods {
			ns[pod.Namespace] = struct{}{}
			Expect(pod.Containers).To(HaveLen(s.containerPerPod), "Container metric count not match")
		}
		Expect(ns).To(HaveLen(s.namespaceCount), "Namespace count not match")
	})
})
