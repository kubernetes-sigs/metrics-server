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

package riemann

import (
	"testing"
	"time"

	pb "github.com/golang/protobuf/proto"
	riemannCommon "github.com/kubernetes-incubator/metrics-server/common/riemann"
	"github.com/kubernetes-incubator/metrics-server/metrics/core"
	"github.com/riemann/riemann-go-client"
	"github.com/riemann/riemann-go-client/proto"
	"github.com/stretchr/testify/assert"
)

type fakeRiemannClient struct {
	events []proto.Event
}

type fakeRiemannSink struct {
	core.DataSink
	fakeRiemannClient *fakeRiemannClient
}

func NewFakeRiemannClient() *fakeRiemannClient {
	return &fakeRiemannClient{[]proto.Event{}}
}

func (client *fakeRiemannClient) Connect(timeout int32) error {
	return nil
}

func (client *fakeRiemannClient) Close() error {
	return nil
}

func (client *fakeRiemannClient) Send(e *proto.Msg) (*proto.Msg, error) {
	msg := &proto.Msg{Ok: pb.Bool(true)}
	for _, event := range e.Events {
		client.events = append(client.events, *event)
	}
	// always returns a Ok msg
	return msg, nil
}

// Returns a fake Riemann sink.
func NewFakeSink() fakeRiemannSink {
	riemannClient := NewFakeRiemannClient()
	c := riemannCommon.RiemannConfig{
		Host:      "riemann-heapster:5555",
		Ttl:       60.0,
		State:     "",
		Tags:      []string{"heapster"},
		BatchSize: 1000,
	}

	return fakeRiemannSink{
		&RiemannSink{
			client: riemannClient,
			config: c,
		},
		riemannClient,
	}
}

func TestAppendEvent(t *testing.T) {
	c := riemannCommon.RiemannConfig{
		Host:      "riemann-heapster:5555",
		Ttl:       60.0,
		State:     "",
		Tags:      make([]string, 0),
		BatchSize: 1000,
	}
	sink := &RiemannSink{
		client: nil,
		config: c,
	}
	var events []riemanngo.Event
	labels := map[string]string{
		"foo": "bar",
	}
	events = appendEvent(events, sink, "riemann", "service1", 10, labels, 1)
	events = appendEvent(events, sink, "riemann", "service1", 10.1, labels, 1)
	assert.Equal(t, 2, len(events))
	assert.Equal(t, events[0], riemanngo.Event{
		Host:    "riemann",
		Service: "service1",
		Metric:  10,
		Time:    1,
		Ttl:     60,
		Tags:    []string{},
		Attributes: map[string]string{
			"foo": "bar",
		},
	})
	assert.Equal(t, events[1], riemanngo.Event{
		Host:    "riemann",
		Service: "service1",
		Metric:  10.1,
		Time:    1,
		Ttl:     60,
		Tags:    []string{},
		Attributes: map[string]string{
			"foo": "bar",
		},
	})
}

func TestAppendEventFull(t *testing.T) {
	riemannClient := NewFakeRiemannClient()
	c := riemannCommon.RiemannConfig{
		Host:      "riemann-heapster:5555",
		Ttl:       60.0,
		State:     "",
		Tags:      make([]string, 0),
		BatchSize: 1000,
	}
	fakeSink := RiemannSink{
		client: riemannClient,
		config: c,
	}

	var events []riemanngo.Event
	for i := 0; i < 999; i++ {
		events = appendEvent(events, &fakeSink, "riemann", "service1", 10, map[string]string{}, 1)
	}
	assert.Equal(t, 999, len(events))
	// batch size = 1000
	events = appendEvent(events, &fakeSink, "riemann", "service1", 10, map[string]string{}, 1)
	assert.Equal(t, 0, len(events))
}

func TestStoreDataEmptyInput(t *testing.T) {
	fakeSink := NewFakeSink()
	dataBatch := core.DataBatch{}
	fakeSink.ExportData(&dataBatch)
	assert.Equal(t, 0, len(fakeSink.fakeRiemannClient.events))
}

func TestStoreMultipleDataInput(t *testing.T) {
	fakeSink := NewFakeSink()
	timestamp := time.Now()

	l := make(map[string]string)
	l["namespace_id"] = "123"
	l[core.LabelHostname.Key] = "riemann"
	l["container_name"] = "/system.slice/-.mount"
	l[core.LabelPodId.Key] = "aaaa-bbbb-cccc-dddd"

	l2 := make(map[string]string)
	l2["namespace_id"] = "123"
	l2[core.LabelHostname.Key] = "riemann"
	l2["container_name"] = "/system.slice/dbus.service"
	l2[core.LabelPodId.Key] = "aaaa-bbbb-cccc-dddd"

	l3 := make(map[string]string)
	l3["namespace_id"] = "123"
	l3[core.LabelHostname.Key] = "riemann"
	l3[core.LabelPodId.Key] = "aaaa-bbbb-cccc-dddd"

	l4 := make(map[string]string)
	l4["namespace_id"] = ""
	l4[core.LabelHostname.Key] = "riemann"
	l4[core.LabelPodId.Key] = "aaaa-bbbb-cccc-dddd"

	l5 := make(map[string]string)
	l5["namespace_id"] = "123"
	l5[core.LabelHostname.Key] = "riemann"
	l5[core.LabelPodId.Key] = "aaaa-bbbb-cccc-dddd"

	metricValue := core.MetricValue{
		IntValue:   int64(10),
		FloatValue: 10.0,
		MetricType: 1,
		ValueType:  0,
	}
	metricSet1 := core.MetricSet{
		Labels: l,
		MetricValues: map[string]core.MetricValue{
			"/system.slice/-.mount//cpu/limit": {
				ValueType:  core.ValueInt64,
				MetricType: core.MetricCumulative,
				IntValue:   123456,
			},
		},
		LabeledMetrics: []core.LabeledMetric{
			{
				"labeledmetric",
				map[string]string{
					"foo": "bar",
				},
				metricValue,
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
			"test/metric/2": {
				ValueType:  core.ValueInt64,
				MetricType: core.MetricCumulative,
				IntValue:   12345,
			},
		},
	}

	metricSet5 := core.MetricSet{
		Labels: l5,
		MetricValues: map[string]core.MetricValue{
			"removeme": {
				ValueType:  core.ValueInt64,
				MetricType: core.MetricCumulative,
				IntValue:   123456,
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

	timeValue := timestamp.Unix()
	fakeSink.ExportData(&data)

	assert.Equal(t, 6, len(fakeSink.fakeRiemannClient.events))
	var expectedEvents = []*proto.Event{
		{
			Host:         pb.String("riemann"),
			Time:         pb.Int64(timeValue),
			Ttl:          pb.Float32(60),
			MetricSint64: pb.Int64(10),
			Service:      pb.String("labeledmetric"),
			Tags:         []string{"heapster"},
			Attributes: []*proto.Attribute{
				{
					Key:   pb.String("container_name"),
					Value: pb.String("/system.slice/-.mount"),
				},
				{
					Key:   pb.String("foo"),
					Value: pb.String("bar"),
				},
				{
					Key:   pb.String("hostname"),
					Value: pb.String("riemann"),
				},
				{
					Key:   pb.String("namespace_id"),
					Value: pb.String("123"),
				},
				{
					Key:   pb.String("pod_id"),
					Value: pb.String("aaaa-bbbb-cccc-dddd"),
				},
			},
		},
		{
			Host:         pb.String("riemann"),
			Time:         pb.Int64(timeValue),
			Ttl:          pb.Float32(60),
			MetricSint64: pb.Int64(123456),
			Service:      pb.String("/system.slice/-.mount//cpu/limit"),
			Tags:         []string{"heapster"},
			Attributes: []*proto.Attribute{
				{
					Key:   pb.String("container_name"),
					Value: pb.String("/system.slice/-.mount"),
				},
				{
					Key:   pb.String("hostname"),
					Value: pb.String("riemann"),
				},
				{
					Key:   pb.String("namespace_id"),
					Value: pb.String("123"),
				},
				{
					Key:   pb.String("pod_id"),
					Value: pb.String("aaaa-bbbb-cccc-dddd"),
				},
			},
		},
		{
			Host:         pb.String("riemann"),
			Time:         pb.Int64(timeValue),
			Ttl:          pb.Float32(60),
			MetricSint64: pb.Int64(123456),
			Service:      pb.String("/system.slice/dbus.service//cpu/usage"),
			Tags:         []string{"heapster"},
			Attributes: []*proto.Attribute{
				{
					Key:   pb.String("container_name"),
					Value: pb.String("/system.slice/dbus.service"),
				},
				{
					Key:   pb.String("hostname"),
					Value: pb.String("riemann"),
				},
				{
					Key:   pb.String("namespace_id"),
					Value: pb.String("123"),
				},
				{
					Key:   pb.String("pod_id"),
					Value: pb.String("aaaa-bbbb-cccc-dddd"),
				},
			},
		},
		{
			Host:         pb.String("riemann"),
			Time:         pb.Int64(timeValue),
			Ttl:          pb.Float32(60),
			MetricSint64: pb.Int64(123456),
			Service:      pb.String("test/metric/1"),
			Tags:         []string{"heapster"},
			Attributes: []*proto.Attribute{
				{
					Key:   pb.String("hostname"),
					Value: pb.String("riemann"),
				},
				{
					Key:   pb.String("namespace_id"),
					Value: pb.String("123"),
				},
				{
					Key:   pb.String("pod_id"),
					Value: pb.String("aaaa-bbbb-cccc-dddd"),
				},
			},
		},
		{
			Host:         pb.String("riemann"),
			Time:         pb.Int64(timeValue),
			Ttl:          pb.Float32(60),
			MetricSint64: pb.Int64(12345),
			Service:      pb.String("test/metric/2"),
			Tags:         []string{"heapster"},
			Attributes: []*proto.Attribute{
				{
					Key:   pb.String("hostname"),
					Value: pb.String("riemann"),
				},
				{
					Key:   pb.String("namespace_id"),
					Value: pb.String(""),
				},
				{
					Key:   pb.String("pod_id"),
					Value: pb.String("aaaa-bbbb-cccc-dddd"),
				},
			},
		},
		{
			Host:         pb.String("riemann"),
			Time:         pb.Int64(timeValue),
			Ttl:          pb.Float32(60),
			MetricSint64: pb.Int64(123456),
			Service:      pb.String("removeme"),
			Tags:         []string{"heapster"},
			Attributes: []*proto.Attribute{
				{
					Key:   pb.String("hostname"),
					Value: pb.String("riemann"),
				},
				{
					Key:   pb.String("namespace_id"),
					Value: pb.String("123"),
				},
				{
					Key:   pb.String("pod_id"),
					Value: pb.String("aaaa-bbbb-cccc-dddd"),
				},
			},
		},
	}

	for _, expectedEvent := range expectedEvents {
		found := false
		for _, sinkEvent := range fakeSink.fakeRiemannClient.events {
			if pb.Equal(expectedEvent, &sinkEvent) {
				found = true
			}
		}
		if !found {
			t.Error("Error, event not found in sink")
		}
	}
}
