// Copyright 2018 The Kubernetes Authors.
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

package scraper

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sigs.k8s.io/metrics-server/pkg/scraper/client"

	corev1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/component-base/metrics"
	"k8s.io/klog/v2"

	"sigs.k8s.io/metrics-server/pkg/storage"
)

var (
	requestDuration = metrics.NewHistogramVec(
		&metrics.HistogramOpts{
			Namespace: "metrics_server",
			Subsystem: "kubelet",
			Name:      "request_duration_seconds",
			Help:      "Duration of requests to Kubelet API in seconds",
			Buckets:   metrics.DefBuckets,
		},
		[]string{"node"},
	)
	requestTotal = metrics.NewCounterVec(
		&metrics.CounterOpts{
			Namespace: "metrics_server",
			Subsystem: "kubelet",
			Name:      "request_total",
			Help:      "Number of requests sent to Kubelet API",
		},
		[]string{"success"},
	)
	lastRequestTime = metrics.NewGaugeVec(
		&metrics.GaugeOpts{
			Namespace: "metrics_server",
			Subsystem: "kubelet",
			Name:      "last_request_time_seconds",
			Help:      "Time of last request performed to Kubelet API since unix epoch in seconds",
		},
		[]string{"node"},
	)
)

// RegisterScraperMetrics registers rate, errors, and duration metrics on
// Kubelet API scrapes.
func RegisterScraperMetrics(registrationFunc func(metrics.Registerable) error) error {
	for _, metric := range []metrics.Registerable{
		requestDuration,
		requestTotal,
		lastRequestTime,
	} {
		err := registrationFunc(metric)
		if err != nil {
			return err
		}
	}
	return nil
}

func NewScraper(client client.KubeletMetricsGetter, scrapeTimeout time.Duration, metricResolution time.Duration, store storage.Storage) *manageNodeScraper {
	return &manageNodeScraper{
		kubeletClient:    client,
		scrapeTimeout:    scrapeTimeout,
		metricResolution: metricResolution,
		stop:             map[apitypes.UID]chan struct{}{},
		isWorking:        map[apitypes.UID]bool{},
		storage:          store,
		nodePodsMap:      make(map[string][]apitypes.NamespacedName),
	}
}

type manageNodeScraper struct {
	nodeLock         sync.RWMutex
	kubeletClient    client.KubeletMetricsGetter
	scrapeTimeout    time.Duration
	metricResolution time.Duration
	// Tracks all running per-node goroutines - per-node goroutine will be
	// processing updates received through its corresponding channel.
	stop map[apitypes.UID]chan struct{}
	// Track the current state of per-node goroutines.
	// Currently all update request for a given node coming when another
	// update of this node is being processed are ignored.
	isWorking map[apitypes.UID]bool

	storage         storage.Storage
	nodePodsMap     map[string][]apitypes.NamespacedName
	nodePodsMapLock sync.Mutex
}

// NodeInfo contains the information needed to identify and connect to a particular node
// (node name and preferred address).
type NodeInfo struct {
	Name           string
	ConnectAddress string
}

func (m *manageNodeScraper) AddNodeScraper(node *corev1.Node) error {
	m.nodeLock.Lock()
	defer m.nodeLock.Unlock()
	if working, exists := m.isWorking[node.UID]; exists && working {
		klog.V(1).InfoS("Scrape in node is already running", "node", klog.KObj(node))
		return fmt.Errorf("Scrape in node is already running, node:%v", node)
	}
	klog.V(1).InfoS("Start scrape metrics from node", "node", klog.KObj(node))
	stopCh, exists := m.stop[node.UID]
	if !exists {
		stopCh = make(chan struct{})
		m.stop[node.UID] = stopCh
	}
	go func() {
		ticker := time.NewTicker(m.metricResolution)
		defer ticker.Stop()
		m.processData(node)
		for {
			select {
			case <-ticker.C:
				m.processData(node)
			case <-stopCh:
				klog.V(1).InfoS("Scrape metrics from node exit", "node", klog.KObj(node))
				return
			}
		}
	}()
	if !m.isWorking[node.UID] {
		m.isWorking[node.UID] = true
	}
	return nil
}
func (m *manageNodeScraper) ScrapeData(node *corev1.Node) (*storage.MetricsBatch, error) {
	startTime := myClock.Now()
	klog.V(6).InfoS("Scraping metrics from node", "node", klog.KObj(node))
	ctx, cancel := context.WithTimeout(context.Background(), m.scrapeTimeout)
	defer cancel()
	defer func() {
		requestDuration.WithLabelValues(node.Name).Observe(float64(myClock.Since(startTime)) / float64(time.Second))
		lastRequestTime.WithLabelValues(node.Name).Set(float64(myClock.Now().Unix()))
	}()
	ms, err := m.kubeletClient.GetMetrics(ctx, node)
	m.nodePodsMapLock.Lock()
	defer m.nodePodsMapLock.Unlock()
	if err != nil {
		klog.ErrorS(err, "Failed to scrape node", "node", klog.KObj(node))
		requestTotal.WithLabelValues("false").Inc()
		return nil, fmt.Errorf("Failed to scrape node: %v", node)
	}
	requestTotal.WithLabelValues("true").Inc()
	for podRef := range ms.Pods {
		m.nodePodsMap[node.Name] = append(m.nodePodsMap[node.Name], podRef)
	}
	return ms, nil
}
func (m *manageNodeScraper) StoreData(node *corev1.Node, res *storage.MetricsBatch) {
	klog.V(6).InfoS("Storing metrics from node", "node", klog.KObj(node))
	m.storage.Store(res)
}
func (m *manageNodeScraper) processData(node *corev1.Node) {
	res, err := m.ScrapeData(node)
	if err == nil {
		m.StoreData(node, res)
	} else {
		klog.V(1).InfoS("Scrape no value from node", "node", klog.KObj(node), "err", err)
	}
}
func (m *manageNodeScraper) DeleteNodeScraper(node *corev1.Node) {
	m.nodeLock.RLock()
	isWorking := m.isWorking[node.UID]
	m.nodeLock.RUnlock()
	if !isWorking {
		return
	}

	klog.V(1).InfoS("Stop scrape metrics from node", "node", klog.KObj(node))
	m.nodeLock.Lock()
	delete(m.isWorking, node.UID)
	if stop, exists := m.stop[node.UID]; exists {
		stop <- struct{}{}
		delete(m.stop, node.UID)
	}
	m.nodeLock.Unlock()

	m.nodePodsMapLock.Lock()
	if _, found := m.nodePodsMap[node.Name]; found {
		m.storage.DiscardNode(*node)
		m.storage.DiscardPods(m.nodePodsMap[node.Name])
		delete(m.nodePodsMap, node.Name)
	}
	m.nodePodsMapLock.Unlock()
}

type clock interface {
	Now() time.Time
	Since(time.Time) time.Duration
}

type realClock struct{}

func (realClock) Now() time.Time                  { return time.Now() }
func (realClock) Since(d time.Time) time.Duration { return time.Since(d) }

var myClock clock = &realClock{}
