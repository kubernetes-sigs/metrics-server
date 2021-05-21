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

package storage

import (
	"time"

	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/metrics"

	"sigs.k8s.io/metrics-server/pkg/api"
)

// fresh new container's minimum allowable time duration between start time and timestamp.
//if time duration less than 10s, can produce inaccurate data
const freshContainerMinMetricsResolution = 10 * time.Second

// nodeStorage stores last two pod metric batches and calculates cpu & memory usage
//
// This implementation only stores metric points if they are newer than the
// points already stored and the cpuUsageOverTime function used to handle
// cumulative metrics assumes that the time window is different from 0.
type podStorage struct {
	// last stores pod metric points from last scrape
	last map[apitypes.NamespacedName]map[string]ContainerMetricsPoint
	// prev stores pod metric points from scrape preceding the last one.
	// Points timestamp should proceed the corresponding points from last and have same start time (no restart between them).
	prev map[apitypes.NamespacedName]map[string]ContainerMetricsPoint
	//scrape period of metrics server
	metricResolution time.Duration
}

func (s *podStorage) GetMetrics(pods ...apitypes.NamespacedName) ([]api.TimeInfo, [][]metrics.ContainerMetrics, error) {
	tis := make([]api.TimeInfo, len(pods))
	ms := make([][]metrics.ContainerMetrics, len(pods))
	for i, pod := range pods {
		lastPod, found := s.last[pod]
		if !found {
			continue
		}

		prevPod, found := s.prev[pod]
		if !found {
			continue
		}

		var (
			cms              = make([]metrics.ContainerMetrics, 0, len(lastPod))
			earliestTimeInfo api.TimeInfo
		)
		for container, lastContainer := range lastPod {
			prevContainer, found := prevPod[container]
			if !found {
				continue
			}
			usage, ti, err := resourceUsage(lastContainer.MetricsPoint, prevContainer.MetricsPoint)
			if err != nil {
				klog.ErrorS(err, "Skipping container usage metric", "container", container, "pod", klog.KRef(pod.Namespace, pod.Name))
				continue
			}
			cms = append(cms, metrics.ContainerMetrics{
				Name:  lastContainer.Name,
				Usage: usage,
			})
			if earliestTimeInfo.Timestamp.IsZero() || earliestTimeInfo.Timestamp.After(ti.Timestamp) {
				earliestTimeInfo = ti
			}
		}
		ms[i] = cms
		tis[i] = earliestTimeInfo
	}
	return tis, ms, nil
}

func (s *podStorage) Store(newPods []PodMetricsPoint) {
	lastPods := make(map[apitypes.NamespacedName]map[string]ContainerMetricsPoint, len(newPods))
	prevPods := make(map[apitypes.NamespacedName]map[string]ContainerMetricsPoint, len(newPods))
	var containerCount int
	for _, newPod := range newPods {
		podRef := apitypes.NamespacedName{Name: newPod.Name, Namespace: newPod.Namespace}
		if _, found := lastPods[podRef]; found {
			klog.ErrorS(nil, "Got duplicate pod point", "pod", klog.KRef(newPod.Namespace, newPod.Name))
			continue
		}

		lastContainers := make(map[string]ContainerMetricsPoint, len(newPod.Containers))
		prevContainers := make(map[string]ContainerMetricsPoint, len(newPod.Containers))
		for _, newContainer := range newPod.Containers {
			if _, exists := lastContainers[newContainer.Name]; exists {
				klog.ErrorS(nil, "Got duplicate Container point", "container", newContainer.Name, "pod", klog.KRef(newPod.Namespace, newPod.Name))
				continue
			}
			lastContainers[newContainer.Name] = newContainer
			if newContainer.StartTime.Before(newContainer.Timestamp) && newContainer.Timestamp.Sub(newContainer.StartTime) < s.metricResolution && newContainer.Timestamp.Sub(newContainer.StartTime) >= freshContainerMinMetricsResolution {
				copied := newContainer
				copied.MetricsPoint.Timestamp = newContainer.StartTime
				copied.MetricsPoint.CumulativeCpuUsed = 0
				prevContainers[newContainer.Name] = copied
			} else if lastPod, found := s.last[podRef]; found {
				// Keep previous metric point if newContainer has not restarted (new metric start time < stored timestamp)
				if lastContainer, found := lastPod[newContainer.Name]; found && newContainer.StartTime.Before(lastContainer.Timestamp) {
					// If new point is different then one already stored
					if newContainer.Timestamp.After(lastContainer.Timestamp) {
						// Move stored point to previous
						prevContainers[newContainer.Name] = lastContainer
					} else if prevPod, found := s.prev[podRef]; found {
						if prevPod[newContainer.Name].Timestamp.Before(newContainer.Timestamp) {
							// Keep previous point
							prevContainers[newContainer.Name] = prevPod[newContainer.Name]
						} else {
							klog.V(2).InfoS("Found new container metrics point is older then stored previous , drop previous",
								"container", newContainer.Name,
								"pod", klog.KRef(newPod.Namespace, newPod.Name),
								"previousTimestamp", prevPod[newContainer.Name].Timestamp,
								"timestamp", newContainer.Timestamp)
						}
					}
				}
			}
		}
		containerPoints := len(prevContainers)
		if containerPoints > 0 {
			prevPods[podRef] = prevContainers
		}
		lastPods[podRef] = lastContainers

		// Only count containers for which metrics can be returned.
		containerCount += containerPoints
	}
	s.last = lastPods
	s.prev = prevPods

	pointsStored.WithLabelValues("container").Set(float64(containerCount))
}
