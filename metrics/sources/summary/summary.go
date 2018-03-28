// Copyright 2015 Google Inc. All Rights Reserved.
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
	"fmt"
	"time"

	. "github.com/kubernetes-incubator/metrics-server/metrics/core"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/metrics-server/metrics/util"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

var (
	summaryRequestLatency = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: "heapster",
			Subsystem: "kubelet_summary",
			Name:      "request_duration_microseconds",
			Help:      "The Kubelet summary request latencies in microseconds.",
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

// Prefix used for the LabelResourceID for volume metrics.
const VolumeResourcePrefix = "Volume:"

func init() {
	prometheus.MustRegister(summaryRequestLatency)
	prometheus.MustRegister(scrapeTotal)
}

type NodeInfo struct {
	IP             string
	NodeName       string
	HostName       string
	HostID         string
	KubeletVersion string
}

// Kubelet-provided metrics for pod and system container.
type summaryMetricsSource struct {
	node          NodeInfo
	kubeletClient KubeletInterface
}

func NewSummaryMetricsSource(node NodeInfo, client KubeletInterface) MetricsSource {
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

func (src *summaryMetricsSource) ScrapeMetrics(start, end time.Time) *DataBatch {
	result := &DataBatch{
		Timestamp:  time.Now(),
		MetricSets: map[string]*MetricSet{},
	}

	summary, err := func() (*stats.Summary, error) {
		startTime := time.Now()
		defer summaryRequestLatency.WithLabelValues(src.node.HostName).Observe(float64(time.Since(startTime)))
		return src.kubeletClient.GetSummary(src.node.IP)
	}()

	if err != nil {
		glog.Errorf("error while getting metrics summary from Kubelet %s(%s): %v", src.node.NodeName, src.node.IP, err)
		scrapeTotal.WithLabelValues("false").Inc()
		return result
	}

	scrapeTotal.WithLabelValues("true").Inc()
	result.MetricSets = src.decodeSummary(summary)

	return result
}

const (
	RootFsKey = "/"
	LogsKey   = "logs"
)

// For backwards compatibility, map summary system names into original names.
// TODO: Migrate to the new system names and remove src.
var systemNameMap = map[string]string{
	stats.SystemContainerRuntime: "docker-daemon",
	stats.SystemContainerMisc:    "system",
}

// decodeSummary translates the kubelet stats.Summary API into the flattened heapster MetricSet API.
func (src *summaryMetricsSource) decodeSummary(summary *stats.Summary) map[string]*MetricSet {
	glog.V(9).Infof("Begin summary decode")
	result := map[string]*MetricSet{}

	labels := map[string]string{
		LabelNodename.Key: src.node.NodeName,
		LabelHostname.Key: src.node.HostName,
		LabelHostID.Key:   src.node.HostID,
	}

	src.decodeNodeStats(result, labels, &summary.Node)
	for _, pod := range summary.Pods {
		src.decodePodStats(result, labels, &pod)
	}

	glog.V(9).Infof("End summary decode")
	return result
}

// Convenience method for labels deep copy.
func (src *summaryMetricsSource) cloneLabels(labels map[string]string) map[string]string {
	clone := make(map[string]string, len(labels))
	for k, v := range labels {
		clone[k] = v
	}
	return clone
}

func (src *summaryMetricsSource) decodeNodeStats(metrics map[string]*MetricSet, labels map[string]string, node *stats.NodeStats) {
	glog.V(9).Infof("Decoding node stats for node %s...", node.NodeName)
	nodeMetrics := &MetricSet{
		Labels:         src.cloneLabels(labels),
		MetricValues:   map[string]MetricValue{},
		LabeledMetrics: []LabeledMetric{},
		CreateTime:     node.StartTime.Time,
		ScrapeTime:     src.getScrapeTime(node.CPU, node.Memory, node.Network),
	}
	nodeMetrics.Labels[LabelMetricSetType.Key] = MetricSetTypeNode

	src.decodeUptime(nodeMetrics, node.StartTime.Time)
	src.decodeCPUStats(nodeMetrics, node.CPU)
	src.decodeMemoryStats(nodeMetrics, node.Memory)
	src.decodeNetworkStats(nodeMetrics, node.Network)
	src.decodeFsStats(nodeMetrics, RootFsKey, node.Fs)
	metrics[NodeKey(node.NodeName)] = nodeMetrics

	for _, container := range node.SystemContainers {
		key := NodeContainerKey(node.NodeName, src.getContainerName(&container))
		containerMetrics := src.decodeContainerStats(labels, &container)
		containerMetrics.Labels[LabelMetricSetType.Key] = MetricSetTypeSystemContainer
		metrics[key] = containerMetrics
	}
}

func (src *summaryMetricsSource) decodePodStats(metrics map[string]*MetricSet, nodeLabels map[string]string, pod *stats.PodStats) {
	glog.V(9).Infof("Decoding pod stats for pod %s/%s (%s)...", pod.PodRef.Namespace, pod.PodRef.Name, pod.PodRef.UID)
	podMetrics := &MetricSet{
		Labels:         src.cloneLabels(nodeLabels),
		MetricValues:   map[string]MetricValue{},
		LabeledMetrics: []LabeledMetric{},
		CreateTime:     pod.StartTime.Time,
		ScrapeTime:     src.getScrapeTime(nil, nil, pod.Network),
	}
	ref := pod.PodRef
	podMetrics.Labels[LabelMetricSetType.Key] = MetricSetTypePod
	podMetrics.Labels[LabelPodId.Key] = ref.UID
	podMetrics.Labels[LabelPodName.Key] = ref.Name
	podMetrics.Labels[LabelNamespaceName.Key] = ref.Namespace

	src.decodeUptime(podMetrics, pod.StartTime.Time)
	src.decodeNetworkStats(podMetrics, pod.Network)
	for _, vol := range pod.VolumeStats {
		src.decodeFsStats(podMetrics, VolumeResourcePrefix+vol.Name, &vol.FsStats)
	}
	metrics[PodKey(ref.Namespace, ref.Name)] = podMetrics

	for _, container := range pod.Containers {
		key := PodContainerKey(ref.Namespace, ref.Name, container.Name)
		metrics[key] = src.decodeContainerStats(podMetrics.Labels, &container)
	}
}

func (src *summaryMetricsSource) decodeContainerStats(podLabels map[string]string, container *stats.ContainerStats) *MetricSet {
	glog.V(9).Infof("Decoding container stats stats for container %s...", container.Name)
	containerMetrics := &MetricSet{
		Labels:         src.cloneLabels(podLabels),
		MetricValues:   map[string]MetricValue{},
		LabeledMetrics: []LabeledMetric{},
		CreateTime:     container.StartTime.Time,
		ScrapeTime:     src.getScrapeTime(container.CPU, container.Memory, nil),
	}
	containerMetrics.Labels[LabelMetricSetType.Key] = MetricSetTypePodContainer
	containerMetrics.Labels[LabelContainerName.Key] = src.getContainerName(container)

	src.decodeUptime(containerMetrics, container.StartTime.Time)
	src.decodeCPUStats(containerMetrics, container.CPU)
	src.decodeMemoryStats(containerMetrics, container.Memory)
	src.decodeFsStats(containerMetrics, RootFsKey, container.Rootfs)
	src.decodeFsStats(containerMetrics, LogsKey, container.Logs)
	src.decodeUserDefinedMetrics(containerMetrics, container.UserDefinedMetrics)

	return containerMetrics
}

func (src *summaryMetricsSource) decodeUptime(metrics *MetricSet, startTime time.Time) {
	if startTime.IsZero() {
		glog.V(9).Infof("missing start time!")
		return
	}

	uptime := uint64(time.Since(startTime).Nanoseconds() / time.Millisecond.Nanoseconds())
	src.addIntMetric(metrics, &MetricUptime, &uptime)
}

func (src *summaryMetricsSource) decodeCPUStats(metrics *MetricSet, cpu *stats.CPUStats) {
	if cpu == nil {
		glog.V(9).Infof("missing cpu usage metric!")
		return
	}

	src.addIntMetric(metrics, &MetricCpuUsage, cpu.UsageCoreNanoSeconds)
}

func (src *summaryMetricsSource) decodeMemoryStats(metrics *MetricSet, memory *stats.MemoryStats) {
	if memory == nil {
		glog.V(9).Infof("missing memory metrics!")
		return
	}

	src.addIntMetric(metrics, &MetricMemoryUsage, memory.UsageBytes)
	src.addIntMetric(metrics, &MetricMemoryWorkingSet, memory.WorkingSetBytes)
	src.addIntMetric(metrics, &MetricMemoryPageFaults, memory.PageFaults)
	src.addIntMetric(metrics, &MetricMemoryMajorPageFaults, memory.MajorPageFaults)
}

func (src *summaryMetricsSource) decodeNetworkStats(metrics *MetricSet, network *stats.NetworkStats) {
	if network == nil {
		glog.V(9).Infof("missing network metrics!")
		return
	}

	src.addIntMetric(metrics, &MetricNetworkRx, network.RxBytes)
	src.addIntMetric(metrics, &MetricNetworkRxErrors, network.RxErrors)
	src.addIntMetric(metrics, &MetricNetworkTx, network.TxBytes)
	src.addIntMetric(metrics, &MetricNetworkTxErrors, network.TxErrors)
}

func (src *summaryMetricsSource) decodeFsStats(metrics *MetricSet, fsKey string, fs *stats.FsStats) {
	if fs == nil {
		glog.V(9).Infof("missing fs metrics!")
		return
	}

	fsLabels := map[string]string{LabelResourceID.Key: fsKey}
	src.addLabeledIntMetric(metrics, &MetricFilesystemUsage, fsLabels, fs.UsedBytes)
	src.addLabeledIntMetric(metrics, &MetricFilesystemLimit, fsLabels, fs.CapacityBytes)
	src.addLabeledIntMetric(metrics, &MetricFilesystemAvailable, fsLabels, fs.AvailableBytes)
}

func (src *summaryMetricsSource) decodeUserDefinedMetrics(metrics *MetricSet, udm []stats.UserDefinedMetric) {
	for _, metric := range udm {
		mv := MetricValue{}
		switch metric.Type {
		case stats.MetricGauge:
			mv.MetricType = MetricGauge
		case stats.MetricCumulative:
			mv.MetricType = MetricCumulative
		case stats.MetricDelta:
			mv.MetricType = MetricDelta
		default:
			glog.V(4).Infof("Skipping %s: unknown custom metric type: %v", metric.Name, metric.Type)
			continue
		}

		// TODO: Handle double-precision values.
		mv.ValueType = ValueFloat
		mv.FloatValue = float32(metric.Value)

		metrics.MetricValues[CustomMetricPrefix+metric.Name] = mv
	}
}

func (src *summaryMetricsSource) getScrapeTime(cpu *stats.CPUStats, memory *stats.MemoryStats, network *stats.NetworkStats) time.Time {
	// Assume CPU, memory and network scrape times are the same.
	switch {
	case cpu != nil && !cpu.Time.IsZero():
		return cpu.Time.Time
	case memory != nil && !memory.Time.IsZero():
		return memory.Time.Time
	case network != nil && !network.Time.IsZero():
		return network.Time.Time
	default:
		return time.Time{}
	}
}

// addIntMetric is a convenience method for adding the metric and value to the metric set.
func (src *summaryMetricsSource) addIntMetric(metrics *MetricSet, metric *Metric, value *uint64) {
	if value == nil {
		glog.V(9).Infof("skipping metric %s because the value was nil", metric.Name)
		return
	}
	val := MetricValue{
		ValueType:  ValueInt64,
		MetricType: metric.Type,
		IntValue:   int64(*value),
	}
	metrics.MetricValues[metric.Name] = val
}

// addLabeledIntMetric is a convenience method for adding the labeled metric and value to the metric set.
func (src *summaryMetricsSource) addLabeledIntMetric(metrics *MetricSet, metric *Metric, labels map[string]string, value *uint64) {
	if value == nil {
		glog.V(9).Infof("skipping labeled metric %s (%v) because the value was nil", metric.Name, labels)
		return
	}

	val := LabeledMetric{
		Name:   metric.Name,
		Labels: labels,
		MetricValue: MetricValue{
			ValueType:  ValueInt64,
			MetricType: metric.Type,
			IntValue:   int64(*value),
		},
	}
	metrics.LabeledMetrics = append(metrics.LabeledMetrics, val)
}

// Translate system container names to the legacy names for backwards compatibility.
func (src *summaryMetricsSource) getContainerName(c *stats.ContainerStats) string {
	if legacyName, ok := systemNameMap[c.Name]; ok {
		return legacyName
	}
	return c.Name
}

// TODO: The summaryProvider duplicates a lot of code from kubeletProvider, and should be refactored.
type summaryProvider struct {
	nodeLister    v1listers.NodeLister
	reflector     *cache.Reflector
	kubeletClient KubeletInterface
}

func (p *summaryProvider) GetMetricsSources() []MetricsSource {
	sources := []MetricsSource{}
	nodes, err := p.nodeLister.List(labels.Everything())
	if err != nil {
		glog.Errorf("error while listing nodes: %v", err)
		return sources
	}

	for _, node := range nodes {
		info, err := p.getNodeInfo(node)
		if err != nil {
			glog.Errorf("%v", err)
			continue
		}
		sources = append(sources, NewSummaryMetricsSource(info, p.kubeletClient))
	}
	return sources
}

func (p *summaryProvider) getNodeInfo(node *corev1.Node) (NodeInfo, error) {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady && c.Status != corev1.ConditionTrue {
			return NodeInfo{}, fmt.Errorf("Node %v is not ready", node.Name)
		}
	}
	info := NodeInfo{
		NodeName:       node.Name,
		HostName:       node.Name,
		HostID:         node.Spec.ExternalID,
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

func NewSummaryProvider(restClient rest.Interface, kubeletClient KubeletInterface) (MetricsSourceProvider, error) {
	// watch nodes
	nodeLister, reflector, _ := util.GetNodeLister(restClient)

	return &summaryProvider{
		nodeLister:    nodeLister,
		reflector:     reflector,
		kubeletClient: kubeletClient,
	}, nil
}
