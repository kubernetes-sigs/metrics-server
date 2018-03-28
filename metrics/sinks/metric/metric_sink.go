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

package metric

import (
	"sync"
	"time"

	"github.com/kubernetes-incubator/metrics-server/metrics/core"
)

// A simple in-memory storage for metrics. It divides metrics into 2 categories
// * metrics that need to be stored for couple minutes.
// * metrics that need to be stored for longer time (15 min, 1 hour).
// The user of this struct needs to decide what are the long-stored metrics upfront.
type MetricSink struct {
	// Request can come from other threads.
	lock sync.Mutex

	// List of metrics that will be stored for up to X seconds.
	longStoreMetrics   []string
	longStoreDuration  time.Duration
	shortStoreDuration time.Duration

	// Stores full DataBatch with all metrics and labels.
	shortStore []*core.DataBatch
	// Memory-efficient long/mid term storage for metrics.
	longStore []*multimetricStore
}

// Stores values of a single metrics for different MetricSets.
// Assumes that the user knows what the metric is.
type int64Store map[string]int64

type multimetricStore struct {
	// Timestamp of the batch from which the metrics were taken.
	timestamp time.Time
	// Metric name to int64store with metric values.
	store map[string]int64Store
}

func buildMultimetricStore(metrics []string, batch *core.DataBatch) *multimetricStore {
	store := multimetricStore{
		timestamp: batch.Timestamp,
		store:     make(map[string]int64Store, len(metrics)),
	}
	for _, metric := range metrics {
		store.store[metric] = make(int64Store, len(batch.MetricSets))
	}
	for key, ms := range batch.MetricSets {
		for _, metric := range metrics {
			if metricValue, found := ms.MetricValues[metric]; found {
				metricstore := store.store[metric]
				metricstore[key] = metricValue.IntValue
			}
		}
	}
	return &store
}

func (s *MetricSink) Name() string {
	return "Metric Sink"
}

func (s *MetricSink) Stop() {
	// Do nothing.
}

func (s *MetricSink) ExportData(batch *core.DataBatch) {
	s.lock.Lock()
	defer s.lock.Unlock()

	now := time.Now()
	// TODO: add sorting
	s.longStore = append(popOldStore(s.longStore, now.Add(-s.longStoreDuration)),
		buildMultimetricStore(s.longStoreMetrics, batch))
	s.shortStore = append(popOld(s.shortStore, now.Add(-s.shortStoreDuration)), batch)
}

func (s *MetricSink) GetLatestDataBatch() *core.DataBatch {
	s.lock.Lock()
	defer s.lock.Unlock()

	if len(s.shortStore) == 0 {
		return nil
	}
	return s.shortStore[len(s.shortStore)-1]
}

func (s *MetricSink) GetShortStore() []*core.DataBatch {
	s.lock.Lock()
	defer s.lock.Unlock()

	result := make([]*core.DataBatch, 0, len(s.shortStore))
	for _, batch := range s.shortStore {
		result = append(result, batch)
	}
	return result
}

func (s *MetricSink) GetMetric(metricName string, keys []string, start, end time.Time) map[string][]core.TimestampedMetricValue {
	s.lock.Lock()
	defer s.lock.Unlock()

	useLongStore := false
	for _, longStoreMetric := range s.longStoreMetrics {
		if longStoreMetric == metricName {
			useLongStore = true
		}
	}

	result := make(map[string][]core.TimestampedMetricValue)
	if useLongStore {
		for _, store := range s.longStore {
			// Inclusive start and end.
			if !store.timestamp.Before(start) && !store.timestamp.After(end) {
				substore := store.store[metricName]
				for _, key := range keys {
					if val, found := substore[key]; found {
						result[key] = append(result[key], core.TimestampedMetricValue{
							Timestamp: store.timestamp,
							MetricValue: core.MetricValue{
								IntValue:   val,
								ValueType:  core.ValueInt64,
								MetricType: core.MetricGauge,
							},
						})
					}
				}
			}
		}
	} else {
		for _, batch := range s.shortStore {
			// Inclusive start and end.
			if !batch.Timestamp.Before(start) && !batch.Timestamp.After(end) {
				for _, key := range keys {
					metricSet, found := batch.MetricSets[key]
					if !found {
						continue
					}
					metricValue, found := metricSet.MetricValues[metricName]
					if !found {
						continue
					}
					keyResult, found := result[key]
					if !found {
						keyResult = make([]core.TimestampedMetricValue, 0)
					}
					keyResult = append(keyResult, core.TimestampedMetricValue{
						Timestamp:   batch.Timestamp,
						MetricValue: metricValue,
					})
					result[key] = keyResult
				}
			}
		}
	}
	return result
}

func (s *MetricSink) GetLabeledMetric(metricName string, labels map[string]string, keys []string, start, end time.Time) map[string][]core.TimestampedMetricValue {
	// NB: the long store doesn't store labeled metrics, so it's not relevant here
	result := make(map[string][]core.TimestampedMetricValue)
	for _, batch := range s.shortStore {
		// Inclusive start and end
		if !batch.Timestamp.Before(start) && !batch.Timestamp.After(end) {
			for _, key := range keys {
				metricSet, found := batch.MetricSets[key]
				if !found {
					continue
				}

				for _, labeledMetric := range metricSet.LabeledMetrics {
					if labeledMetric.Name != metricName {
						continue
					}

					if len(labeledMetric.Labels) != len(labels) {
						continue
					}

					labelsMatch := true
					for k, v := range labels {
						if lblMetricVal, ok := labeledMetric.Labels[k]; !ok || lblMetricVal != v {
							labelsMatch = false
							break
						}
					}

					if labelsMatch {
						result[key] = append(result[key], core.TimestampedMetricValue{
							Timestamp:   batch.Timestamp,
							MetricValue: labeledMetric.MetricValue,
						})
					}
				}
			}
		}
	}

	return result
}

func (s *MetricSink) GetMetricNames(key string) []string {
	s.lock.Lock()
	defer s.lock.Unlock()

	metricNames := make(map[string]bool)
	for _, batch := range s.shortStore {
		if set, found := batch.MetricSets[key]; found {
			for key := range set.MetricValues {
				metricNames[key] = true
			}
		}
	}
	result := make([]string, 0, len(metricNames))
	for key := range metricNames {
		result = append(result, key)
	}
	return result
}

func (s *MetricSink) getAllNames(predicate func(ms *core.MetricSet) bool,
	name func(key string, ms *core.MetricSet) string) []string {
	s.lock.Lock()
	defer s.lock.Unlock()

	if len(s.shortStore) == 0 {
		return []string{}
	}

	result := make([]string, 0, 0)
	for key, value := range s.shortStore[len(s.shortStore)-1].MetricSets {
		if predicate(value) {
			result = append(result, name(key, value))
		}
	}
	return result
}

/*
 * For debugging only.
 */
func (s *MetricSink) GetMetricSetKeys() []string {
	return s.getAllNames(
		func(ms *core.MetricSet) bool { return true },
		func(key string, ms *core.MetricSet) string { return key })
}

func (s *MetricSink) GetNodes() []string {
	return s.getAllNames(
		func(ms *core.MetricSet) bool { return ms.Labels[core.LabelMetricSetType.Key] == core.MetricSetTypeNode },
		func(key string, ms *core.MetricSet) string { return ms.Labels[core.LabelHostname.Key] })
}

func (s *MetricSink) GetPods() []string {
	return s.getAllNames(
		func(ms *core.MetricSet) bool { return ms.Labels[core.LabelMetricSetType.Key] == core.MetricSetTypePod },
		func(key string, ms *core.MetricSet) string {
			return ms.Labels[core.LabelNamespaceName.Key] + "/" + ms.Labels[core.LabelPodName.Key]
		})
}

func (s *MetricSink) GetNamespaces() []string {
	return s.getAllNames(
		func(ms *core.MetricSet) bool {
			return ms.Labels[core.LabelMetricSetType.Key] == core.MetricSetTypeNamespace
		},
		func(key string, ms *core.MetricSet) string { return ms.Labels[core.LabelNamespaceName.Key] })
}

func (s *MetricSink) GetPodsFromNamespace(namespace string) []string {
	return s.getAllNames(
		func(ms *core.MetricSet) bool {
			return ms.Labels[core.LabelMetricSetType.Key] == core.MetricSetTypePod &&
				ms.Labels[core.LabelNamespaceName.Key] == namespace
		},
		func(key string, ms *core.MetricSet) string {
			return ms.Labels[core.LabelPodName.Key]
		})
}

func (s *MetricSink) GetContainersForPodFromNamespace(namespace, pod string) []string {
	return s.getAllNames(
		func(ms *core.MetricSet) bool {
			return ms.Labels[core.LabelMetricSetType.Key] == core.MetricSetTypePodContainer &&
				ms.Labels[core.LabelNamespaceName.Key] == namespace &&
				ms.Labels[core.LabelPodName.Key] == pod
		},
		func(key string, ms *core.MetricSet) string {
			return ms.Labels[core.LabelContainerName.Key]
		})
}

func (s *MetricSink) GetSystemContainersFromNode(node string) []string {
	return s.getAllNames(
		func(ms *core.MetricSet) bool {
			return ms.Labels[core.LabelMetricSetType.Key] == core.MetricSetTypeSystemContainer &&
				ms.Labels[core.LabelHostname.Key] == node
		},
		func(key string, ms *core.MetricSet) string {
			return ms.Labels[core.LabelContainerName.Key]
		})
}

func popOld(storage []*core.DataBatch, cutoffTime time.Time) []*core.DataBatch {
	result := make([]*core.DataBatch, 0)
	for _, batch := range storage {
		if batch.Timestamp.After(cutoffTime) {
			result = append(result, batch)
		}
	}
	return result
}

func popOldStore(storages []*multimetricStore, cutoffTime time.Time) []*multimetricStore {
	result := make([]*multimetricStore, 0, len(storages))
	for _, store := range storages {
		if store.timestamp.After(cutoffTime) {
			result = append(result, store)
		}
	}
	return result
}

func NewMetricSink(shortStoreDuration, longStoreDuration time.Duration, longStoreMetrics []string) *MetricSink {
	return &MetricSink{
		longStoreMetrics:   longStoreMetrics,
		longStoreDuration:  longStoreDuration,
		shortStoreDuration: shortStoreDuration,
		longStore:          make([]*multimetricStore, 0),
		shortStore:         make([]*core.DataBatch, 0),
	}
}
