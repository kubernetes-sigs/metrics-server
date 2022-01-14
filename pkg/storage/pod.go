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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/metrics"

	"sigs.k8s.io/metrics-server/pkg/api"
)

// fresh new container's minimum allowable time duration between start time and timestamp.
// if time duration less than 10s, can produce inaccurate data
const freshContainerMinMetricsResolution = 10 * time.Second

// nodeStorage stores last two pod metric batches and calculates cpu & memory usage
//
// This implementation only stores metric points if they are newer than the
// points already stored and the cpuUsageOverTime function used to handle
// cumulative metrics assumes that the time window is different from 0.
type podStorage struct {
	// last stores pod metric points from last scrape
	last map[apitypes.NamespacedName]PodMetricsPoint
	// prev stores pod metric points from scrape preceding the last one.
	// Points timestamp should proceed the corresponding points from last and have same start time (no restart between them).
	prev map[apitypes.NamespacedName]PodMetricsPoint
	// scrape period of metrics server
	metricResolution time.Duration
}

func (s *podStorage) GetMetrics(pods ...*metav1.PartialObjectMetadata) ([]metrics.PodMetrics, error) {
	results := make([]metrics.PodMetrics, 0, len(pods))
	for _, pod := range pods {
		lastPod, found := s.last[apitypes.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}]
		if !found {
			continue
		}

		prevPod, found := s.prev[apitypes.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}]
		if !found {
			continue
		}

		var (
			cms              = make([]metrics.ContainerMetrics, 0, len(lastPod.Containers))
			earliestTimeInfo api.TimeInfo
		)
		allContainersPresent := true
		for container, lastContainer := range lastPod.Containers {
			prevContainer, found := prevPod.Containers[container]
			if !found {
				allContainersPresent = false
				break
			}
			usage, ti, err := resourceUsage(lastContainer, prevContainer)
			if err != nil {
				klog.ErrorS(err, "Skipping container usage metric", "container", container, "pod", klog.KRef(pod.Namespace, pod.Name))
				continue
			}
			cms = append(cms, metrics.ContainerMetrics{
				Name:  container,
				Usage: usage,
			})
			if earliestTimeInfo.Timestamp.IsZero() || earliestTimeInfo.Timestamp.After(ti.Timestamp) {
				earliestTimeInfo = ti
			}
		}
		if allContainersPresent {
			results = append(results, metrics.PodMetrics{
				ObjectMeta: metav1.ObjectMeta{
					Name:              pod.Name,
					Namespace:         pod.Namespace,
					Labels:            pod.Labels,
					CreationTimestamp: metav1.NewTime(time.Now()),
				},
				Timestamp:  metav1.NewTime(earliestTimeInfo.Timestamp),
				Window:     metav1.Duration{Duration: earliestTimeInfo.Window},
				Containers: cms,
			})
		}
	}
	return results, nil
}

func (s *podStorage) Store(newPods *MetricsBatch) {
	lastPods := make(map[apitypes.NamespacedName]PodMetricsPoint, len(newPods.Pods))
	prevPods := make(map[apitypes.NamespacedName]PodMetricsPoint, len(newPods.Pods))
	var containerCount int
	for podRef, newPod := range newPods.Pods {
		podRef := apitypes.NamespacedName{Name: podRef.Name, Namespace: podRef.Namespace}
		if _, found := lastPods[podRef]; found {
			klog.ErrorS(nil, "Got duplicate pod point", "pod", klog.KRef(podRef.Namespace, podRef.Name))
			continue
		}

		newLastPod := PodMetricsPoint{Containers: make(map[string]MetricsPoint, len(newPod.Containers))}
		newPrevPod := PodMetricsPoint{Containers: make(map[string]MetricsPoint, len(newPod.Containers))}
		for containerName, newPoint := range newPod.Containers {
			if _, exists := newLastPod.Containers[containerName]; exists {
				klog.ErrorS(nil, "Got duplicate Container point", "container", containerName, "pod", klog.KRef(podRef.Namespace, podRef.Name))
				continue
			}
			newLastPod.Containers[containerName] = newPoint
			if newPoint.StartTime.Before(newPoint.Timestamp) && newPoint.Timestamp.Sub(newPoint.StartTime) < s.metricResolution && newPoint.Timestamp.Sub(newPoint.StartTime) >= freshContainerMinMetricsResolution {
				copied := newPoint
				copied.Timestamp = newPoint.StartTime
				copied.CumulativeCpuUsed = 0
				newPrevPod.Containers[containerName] = copied
			} else if lastPod, found := s.last[podRef]; found {
				// Keep previous metric point if newPoint has not restarted (new metric start time < stored timestamp)
				if lastContainer, found := lastPod.Containers[containerName]; found && newPoint.StartTime.Before(lastContainer.Timestamp) {
					// If new point is different then one already stored
					if newPoint.Timestamp.After(lastContainer.Timestamp) {
						// Move stored point to previous
						newPrevPod.Containers[containerName] = lastContainer
					} else if prevPod, found := s.prev[podRef]; found {
						if prevPod.Containers[containerName].Timestamp.Before(newPoint.Timestamp) {
							// Keep previous point
							newPrevPod.Containers[containerName] = prevPod.Containers[containerName]
						} else {
							klog.V(2).InfoS("Found new containerName metrics point is older than stored previous , drop previous",
								"containerName", containerName,
								"pod", klog.KRef(podRef.Namespace, podRef.Name),
								"previousTimestamp", prevPod.Containers[containerName].Timestamp,
								"timestamp", newPoint.Timestamp)
						}
					}
				}
			}
		}
		containerPoints := len(newPrevPod.Containers)
		if containerPoints > 0 {
			prevPods[podRef] = newPrevPod
		}
		lastPods[podRef] = newLastPod

		// Only count containers for which metrics can be returned.
		containerCount += containerPoints
	}
	s.last = lastPods
	s.prev = prevPods

	pointsStored.WithLabelValues("container").Set(float64(containerCount))
}
