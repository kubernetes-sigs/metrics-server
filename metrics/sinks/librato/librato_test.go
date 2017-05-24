// Copyright 2017 Google Inc. All Rights Reserved.
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

package librato

import (
	"net/url"
	"testing"
	"time"

	librato_common "github.com/kubernetes-incubator/metrics-server/common/librato"
	"github.com/kubernetes-incubator/metrics-server/metrics/core"
	"github.com/stretchr/testify/assert"
)

type fakeLibratoDataSink struct {
	core.DataSink
	fakeDbClient *librato_common.FakeLibratoClient
}

func newRawLibratoSink() *libratoSink {
	return &libratoSink{
		client: librato_common.FakeClient,
		c:      librato_common.Config,
	}
}

func NewFakeSink() fakeLibratoDataSink {
	return fakeLibratoDataSink{
		newRawLibratoSink(),
		librato_common.FakeClient,
	}
}
func TestStoreDataEmptyInput(t *testing.T) {
	fakeSink := NewFakeSink()
	dataBatch := core.DataBatch{}
	fakeSink.ExportData(&dataBatch)
	assert.Equal(t, 0, len(fakeSink.fakeDbClient.Measurements))
}

func TestStoreMultipleDataInput(t *testing.T) {
	fakeSink := NewFakeSink()
	timestamp := time.Now()

	l := make(map[string]string)
	l["namespace_id"] = "123"
	l["container_name"] = "/system.slice/-.mount"
	l[core.LabelPodId.Key] = "aaaa-bbbb-cccc-dddd"

	l2 := make(map[string]string)
	l2["namespace_id"] = "123"
	l2["container_name"] = "/system.slice/dbus.service"
	l2[core.LabelPodId.Key] = "aaaa-bbbb-cccc-dddd"

	l3 := make(map[string]string)
	l3["namespace_id"] = "123"
	l3[core.LabelPodId.Key] = "aaaa-bbbb-cccc-dddd"

	l4 := make(map[string]string)
	l4["namespace_id"] = ""
	l4[core.LabelPodId.Key] = "aaaa-bbbb-cccc-dddd"

	l5 := make(map[string]string)
	l5["namespace_id"] = "123"
	l5[core.LabelPodId.Key] = "aaaa-bbbb-cccc-dddd"

	metricSet1 := core.MetricSet{
		Labels: l,
		MetricValues: map[string]core.MetricValue{
			"/system.slice/-.mount//cpu/limit": {
				ValueType:  core.ValueInt64,
				MetricType: core.MetricCumulative,
				IntValue:   123456,
			},
		},
	}

	metricSet2 := core.MetricSet{
		Labels: l2,
		MetricValues: map[string]core.MetricValue{
			"/system.slice/dbus.service//cpu/usage": {
				ValueType:  core.ValueInt64,
				MetricType: core.MetricCumulative,
				IntValue:   123456,
			},
		},
	}

	metricSet3 := core.MetricSet{
		Labels: l3,
		MetricValues: map[string]core.MetricValue{
			"test/metric/1": {
				ValueType:  core.ValueInt64,
				MetricType: core.MetricCumulative,
				IntValue:   123456,
			},
		},
	}

	metricSet4 := core.MetricSet{
		Labels: l4,
		MetricValues: map[string]core.MetricValue{
			"test/metric/1": {
				ValueType:  core.ValueInt64,
				MetricType: core.MetricCumulative,
				IntValue:   123456,
			},
		},
	}

	metricSet5 := core.MetricSet{
		Labels: l5,
		MetricValues: map[string]core.MetricValue{
			"removeme": {
				ValueType:  core.ValueFloat,
				MetricType: core.MetricCumulative,
				FloatValue: 1.23456,
			},
		},
	}

	data := core.DataBatch{
		Timestamp: timestamp,
		MetricSets: map[string]*core.MetricSet{
			"pod1": &metricSet1,
			"pod2": &metricSet2,
			"pod3": &metricSet3,
			"pod4": &metricSet4,
			"pod5": &metricSet5,
		},
	}

	fakeSink.ExportData(&data)
	assert.Equal(t, 5, len(fakeSink.fakeDbClient.Measurements))
}

func TestCreateLibratoSink(t *testing.T) {
	stubLibratoURL, err := url.Parse("?username=test&token=my_token")
	assert.NoError(t, err)

	//create influxdb sink
	sink, err := CreateLibratoSink(stubLibratoURL)
	assert.NoError(t, err)

	//check sink name
	assert.Equal(t, sink.Name(), "Librato Sink")
}

func checkMetricVal(expected, actual core.MetricValue) bool {
	if expected.ValueType != actual.ValueType {
		return false
	}

	// only check the relevant value type
	switch expected.ValueType {
	case core.ValueFloat:
		return expected.FloatValue == actual.FloatValue
	case core.ValueInt64:
		return expected.IntValue == actual.IntValue
	default:
		return expected == actual
	}
}
