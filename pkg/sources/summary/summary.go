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
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
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
	IP       string
	NodeName string
	HostName string
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

	var errs []error
	errs = append(errs, src.decodeNodeStats(&summary.Node, &res.Nodes[0])...)
	for i, pod := range summary.Pods {
		errs = append(errs, src.decodePodStats(&pod, &res.Pods[i])...)
	}

	return res, utilerrors.NewAggregate(errs)
}

func (src *summaryMetricsSource) decodeNodeStats(nodeStats *stats.NodeStats, target *sources.NodeMetricsPoint) []error {
	timestamp, err := getScrapeTime(nodeStats.CPU, nodeStats.Memory)
	if err != nil {
		// if we can't get a timestamp, assume bad data in general
		return []error{fmt.Errorf("unable to get valid timestamp for metric point for node %q, discarding data: %v", src.node.NodeName, err)}
	}
	*target = sources.NodeMetricsPoint{
		Name: src.node.NodeName,
		MetricsPoint: sources.MetricsPoint{
			Timestamp: timestamp,
		},
	}
	var errs []error
	if err := decodeCPU(&target.CpuUsage, nodeStats.CPU); err != nil {
		errs = append(errs, fmt.Errorf("unable to get CPU for node %q: %v", src.node.NodeName, err))
	}
	if err := decodeMemory(&target.MemoryUsage, nodeStats.Memory); err != nil {
		errs = append(errs, fmt.Errorf("unable to get memory for node %q: %v", src.node.NodeName, err))
	}
	return errs
}

func (src *summaryMetricsSource) decodePodStats(podStats *stats.PodStats, target *sources.PodMetricsPoint) []error {
	*target = sources.PodMetricsPoint{
		Name:       podStats.PodRef.Name,
		Namespace:  podStats.PodRef.Namespace,
		Containers: make([]sources.ContainerMetricsPoint, len(podStats.Containers)),
	}

	var errs []error
	for i, container := range podStats.Containers {
		timestamp, err := getScrapeTime(container.CPU, container.Memory)
		if err != nil {
			// if we can't get a timestamp, assume bad data in general
			errs = append(errs, fmt.Errorf("unable to get a valid timestamp for metric point for container %q in pod %s/%s on node %q, discarding data: %v", container.Name, target.Namespace, target.Name, src.node.NodeName, err))
			continue
		}
		point := sources.ContainerMetricsPoint{
			Name: container.Name,
			MetricsPoint: sources.MetricsPoint{
				Timestamp: timestamp,
			},
		}
		if err := decodeCPU(&point.CpuUsage, container.CPU); err != nil {
			errs = append(errs, fmt.Errorf("unable to get CPU for container %q in pod %s/%s on node %q: %v", container.Name, target.Namespace, target.Name, src.node.NodeName, err))
		}
		if err := decodeMemory(&point.MemoryUsage, container.Memory); err != nil {
			errs = append(errs, fmt.Errorf("unable to get memory for container %q in pod %s/%s on node %q: %v", container.Name, target.Namespace, target.Name, src.node.NodeName, err))
		}

		target.Containers[i] = point
	}

	return errs
}

func decodeCPU(target *resource.Quantity, cpuStats *stats.CPUStats) error {
	if cpuStats == nil || cpuStats.UsageNanoCores == nil {
		return fmt.Errorf("missing cpu usage metric")
	}

	*target = *uint64Quantity(*cpuStats.UsageNanoCores, -9)
	return nil
}

func decodeMemory(target *resource.Quantity, memStats *stats.MemoryStats) error {
	if memStats == nil || memStats.WorkingSetBytes == nil {
		return fmt.Errorf("missing memory usage metric")
	}

	*target = *uint64Quantity(*memStats.WorkingSetBytes, 0)
	target.Format = resource.BinarySI

	return nil
}

func getScrapeTime(cpu *stats.CPUStats, memory *stats.MemoryStats) (time.Time, error) {
	// Assume CPU and memory scrape times are the same
	switch {
	case cpu != nil && !cpu.Time.IsZero():
		return cpu.Time.Time, nil
	case memory != nil && !memory.Time.IsZero():
		return memory.Time.Time, nil
	default:
		return time.Time{}, fmt.Errorf("no non-zero timestamp on either CPU or memory")
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

	glog.V(1).Infof("unexpectedly large resource value %v, loosing precision to fit in scaled resource.Quantity", val)

	// otherwise, lose an decimal order-of-magnitude precision,
	// so we can fit into a scaled quantity
	return resource.NewScaledQuantity(int64(val/10), resource.Scale(1)+scale)
}

type summaryProvider struct {
	nodeLister    v1listers.NodeLister
	kubeletClient KubeletInterface
}

func (p *summaryProvider) GetMetricSources() ([]sources.MetricSource, error) {
	sources := []sources.MetricSource{}
	nodes, err := p.nodeLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("unable to list nodes: %v", err)
	}

	var errs []error
	for _, node := range nodes {
		info, err := p.getNodeInfo(node)
		if err != nil {
			errs = append(errs, fmt.Errorf("unable to extract connection information for node %q: %v", node.Name, err))
			continue
		}
		sources = append(sources, NewSummaryMetricsSource(info, p.kubeletClient))
	}
	return sources, utilerrors.NewAggregate(errs)
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
		NodeName: node.Name,
		HostName: node.Name,
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
