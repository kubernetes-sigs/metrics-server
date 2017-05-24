package riemanngo

import (
	"fmt"
	"os"
	"reflect"
	"time"
	"sort"

	pb "github.com/golang/protobuf/proto"
	"github.com/riemann/riemann-go-client/proto"
)

// convert an event to a protobuf Event
func EventToProtocolBuffer(event *Event) (*proto.Event, error) {
	if event.Host == "" {
		event.Host, _ = os.Hostname()
	}
	if event.Time == 0 {
		event.Time = time.Now().Unix()
	}

	var e proto.Event
	e.Host = pb.String(event.Host)
	e.Time = pb.Int64(event.Time)
	if event.Service != "" {
		e.Service = pb.String(event.Service)
	}

	if event.State != "" {
		e.State = pb.String(event.State)
	}
	if event.Description != "" {
		e.Description = pb.String(event.Description)
	}
	e.Tags = event.Tags
	var attrs []*proto.Attribute

	// sort keys
	var keys []string
	for key := range event.Attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		attrs = append(attrs, &proto.Attribute{
			Key:   pb.String(key),
			Value: pb.String(event.Attributes[key]),
		})
	}
	e.Attributes = attrs
	if event.Ttl != 0 {
		e.Ttl = pb.Float32(event.Ttl)
	}

	switch reflect.TypeOf(event.Metric).Kind() {
	case reflect.Int, reflect.Int32, reflect.Int64:
		e.MetricSint64 = pb.Int64((reflect.ValueOf(event.Metric).Int()))
	case reflect.Float32:
		e.MetricD = pb.Float64((reflect.ValueOf(event.Metric).Float()))
	case reflect.Float64:
		e.MetricD = pb.Float64((reflect.ValueOf(event.Metric).Float()))
	default:
		return nil, fmt.Errorf("Metric of invalid type (type %v)",
			reflect.TypeOf(event.Metric).Kind())
	}
	return &e, nil
}

// converts an array of proto.Event to an array of Event
func ProtocolBuffersToEvents(pbEvents []*proto.Event) []Event {
	var events []Event
	for _, event := range pbEvents {
		e := Event{
			State:       event.GetState(),
			Service:     event.GetService(),
			Host:        event.GetHost(),
			Description: event.GetDescription(),
			Ttl:         event.GetTtl(),
			Time:        event.GetTime(),
			Tags:        event.GetTags(),
		}
		if event.MetricF != nil {
			e.Metric = event.GetMetricF()
		} else if event.MetricD != nil {
			e.Metric = event.GetMetricD()
		} else {
			e.Metric = event.GetMetricSint64()
		}
		if event.Attributes != nil {
			e.Attributes = make(map[string]string, len(event.GetAttributes()))
			for _, attr := range event.GetAttributes() {
				e.Attributes[attr.GetKey()] = attr.GetValue()
			}
		}
		events = append(events, e)
	}
	return events
}
