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

package sinks

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/metrics-server/common/flags"
	"github.com/kubernetes-incubator/metrics-server/metrics/core"
	metricsink "github.com/kubernetes-incubator/metrics-server/metrics/sinks/metric"
)

type SinkFactory struct {
}

func (this *SinkFactory) Build(uri flags.Uri) (core.DataSink, error) {
	switch uri.Key {
	case "metric":
		return metricsink.NewMetricSink(140*time.Second, 15*time.Minute, []string{
			core.MetricCpuUsageRate.MetricDescriptor.Name,
			core.MetricMemoryWorkingSet.MetricDescriptor.Name}), nil
	default:
		return nil, fmt.Errorf("Sink not recognized: %s", uri.Key)
	}
}

func (this *SinkFactory) BuildAll(uris flags.Uris) (*metricsink.MetricSink, []core.DataSink) {
	result := make([]core.DataSink, 0, len(uris))
	var metric *metricsink.MetricSink
	for _, uri := range uris {
		sink, err := this.Build(uri)
		if err != nil {
			glog.Errorf("Failed to create sink: %v", err)
			continue
		}
		if uri.Key == "metric" {
			metric = sink.(*metricsink.MetricSink)
		}
		result = append(result, sink)
	}

	if len([]flags.Uri(uris)) != 0 && len(result) == 0 {
		glog.Fatal("No available sink to use")
	}

	if metric == nil {
		uri := flags.Uri{}
		uri.Set("metric")
		sink, err := this.Build(uri)
		if err == nil {
			result = append(result, sink)
			metric = sink.(*metricsink.MetricSink)
		} else {
			glog.Errorf("Error while creating metric sink: %v", err)
		}
	}
	return metric, result
}

func NewSinkFactory() *SinkFactory {
	return &SinkFactory{}
}
