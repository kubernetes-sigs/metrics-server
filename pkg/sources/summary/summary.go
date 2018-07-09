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

package summary

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/metrics-server/pkg/sources"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	v1listers "k8s.io/client-go/listers/core/v1"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

var (
	summaryRequestLatency = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: "metrics_server",
			Subsystem: "kubelet_summary",
			Name:      "request_duration_seconds",
			Help:      "The Kubelet summary request latencies in seconds.",
		},
		[]string{"node"},
	)
	scrapeTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "metrics_server",
			Subsystem: "kubelet_summary",
			Name:      "scrapes_total",
			Help:      "Total number of attempted Summary API scrapes done by Metrics Server",
		},
		[]string{"success"},
	)
)

func init() {
	prometheus.MustRegister(summaryRequestLatency)
	prometheus.MustRegister(scrapeTotal)
}

type NodeInfo struct {
	IP             string
	NodeName       string
	HostName       string
	KubeletVersion string
}

// Kubelet-provided metrics for pod and system container.
type summaryMetricsSource struct {
	node          NodeInfo
	kubeletClient KubeletInterface
}

func NewSummaryMetricsSource(node NodeInfo, client KubeletInterface) sources.MetricSource {
	return &summaryMetricsSource{
		node:          node,
		kubeletClient: client,
	}
}

func (src *summaryMetricsSource) Name() string {
	return src.String()
}

func (src *summaryMetricsSource) String() string {
	return fmt.Sprintf("kubelet_summary:%s", src.node.IP)
}

func (src *summaryMetricsSource) Collect(ctx context.Context) (*sources.MetricsBatch, error) {
	summary, err := func() (*stats.Summary, error) {
		startTime := time.Now()
		defer summaryRequestLatency.WithLabelValues(src.node.HostName).Observe(float64(time.Since(startTime)) / float64(time.Second))
		return src.kubeletClient.GetSummary(ctx, src.node.IP)
	}()

	if err != nil {
		scrapeTotal.WithLabelValues("false").Inc()
		return nil, fmt.Errorf("unable to fetch metrics from Kubelet %s (%s): %v", src.node.NodeName, src.node.IP, err)
	}

	scrapeTotal.WithLabelValues("true").Inc()

	res := &sources.MetricsBatch{
		Nodes: make([]sources.NodeMetricsPoint, 1),
		Pods:  make([]sources.PodMetricsPoint, len(summary.Pods)),
	}
	src.decodeNodeStats(&summary.Node, &res.Nodes[0])
	for i, pod := range summary.Pods {
		src.decodePodStats(&pod, &res.Pods[i])
	}

	return res, nil
}

func (src *summaryMetricsSource) decodeNodeStats(nodeStats *stats.NodeStats, target *sources.NodeMetricsPoint) {
	*target = sources.NodeMetricsPoint{
		Name: src.node.NodeName,
		MetricsPoint: sources.MetricsPoint{
			Timestamp: getScrapeTime(nodeStats.CPU, nodeStats.Memory),
		},
	}
	decodeCPU(&target.CpuUsage, nodeStats.CPU)
	decodeMemory(&target.MemoryUsage, nodeStats.Memory)
}

func (src *summaryMetricsSource) decodePodStats(podStats *stats.PodStats, target *sources.PodMetricsPoint) {
	*target = sources.PodMetricsPoint{
		Name:       podStats.PodRef.Name,
		Namespace:  podStats.PodRef.Namespace,
		Containers: make([]sources.ContainerMetricsPoint, len(podStats.Containers)),
	}

	for i, container := range podStats.Containers {
		point := sources.ContainerMetricsPoint{
			Name: container.Name,
			MetricsPoint: sources.MetricsPoint{
				Timestamp: getScrapeTime(container.CPU, container.Memory),
			},
		}
		decodeCPU(&point.CpuUsage, container.CPU)
		decodeMemory(&point.MemoryUsage, container.Memory)

		target.Containers[i] = point
	}
}

func decodeCPU(target *resource.Quantity, cpuStats *stats.CPUStats) {
	if cpuStats == nil || cpuStats.UsageNanoCores == nil {
		glog.V(9).Infof("missing cpu usage metric")
		return
	}

	*target = *uint64Quantity(*cpuStats.UsageNanoCores, -9)
}

func decodeMemory(target *resource.Quantity, memStats *stats.MemoryStats) {
	if memStats == nil || memStats.WorkingSetBytes == nil {
		glog.V(9).Infof("missing memory usage metric")
		return
	}

	*target = *uint64Quantity(*memStats.WorkingSetBytes, 0)
	target.Format = resource.BinarySI
}

func getScrapeTime(cpu *stats.CPUStats, memory *stats.MemoryStats) time.Time {
	// Assume CPU, memory and network scrape times are the same.
	switch {
	case cpu != nil && !cpu.Time.IsZero():
		return cpu.Time.Time
	case memory != nil && !memory.Time.IsZero():
		return memory.Time.Time
	default:
		return time.Time{}
	}
}

// uint64Quantity converts a uint64 into a Quantity, which only has constructors
// that work with int64 (except for parse, which requires costly round-trips to string).
// We lose precision until we fit in an int64 if greater than the max int64 value.
func uint64Quantity(val uint64, scale resource.Scale) *resource.Quantity {
	// easy path -- we can safely fit val into an int64
	if val <= math.MaxInt64 {
		return resource.NewScaledQuantity(int64(val), scale)
	}

	// otherwise, lose precision until we can fit into a scaled quantity
	scaleMod := 0
	for val > math.MaxInt64 {
		val /= 10
		scaleMod += 1
	}

	return resource.NewScaledQuantity(int64(val), resource.Scale(scaleMod)+scale)
}

type summaryProvider struct {
	nodeLister    v1listers.NodeLister
	kubeletClient KubeletInterface
}

func (p *summaryProvider) GetMetricSources() []sources.MetricSource {
	sources := []sources.MetricSource{}
	nodes, err := p.nodeLister.List(labels.Everything())
	if err != nil {
		glog.Errorf("error while listing nodes: %v", err)
		return sources
	}

	for _, node := range nodes {
		info, err := p.getNodeInfo(node)
		if err != nil {
			glog.Errorf("error getting node connection information: %v", err)
			continue
		}
		sources = append(sources, NewSummaryMetricsSource(info, p.kubeletClient))
	}
	return sources
}

func (p *summaryProvider) getNodeInfo(node *corev1.Node) (NodeInfo, error) {
	nodeReady := false
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			nodeReady = c.Status == corev1.ConditionTrue
			break
		}
	}
	if !nodeReady {
		return NodeInfo{}, fmt.Errorf("node %v is not ready", node.Name)
	}
	info := NodeInfo{
		NodeName:       node.Name,
		HostName:       node.Name,
		KubeletVersion: node.Status.NodeInfo.KubeletVersion,
	}

	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeHostName && addr.Address != "" {
			info.HostName = addr.Address
		}
		if addr.Type == corev1.NodeInternalIP && addr.Address != "" {
			info.IP = addr.Address
		}
	}

	if info.IP == "" {
		return info, fmt.Errorf("Node %v has no valid hostname and/or IP address: %v %v", node.Name, info.HostName, info.IP)
	}

	return info, nil
}

func NewSummaryProvider(nodeLister v1listers.NodeLister, kubeletClient KubeletInterface) sources.MetricSourceProvider {
	return &summaryProvider{
		nodeLister:    nodeLister,
		kubeletClient: kubeletClient,
	}
}
