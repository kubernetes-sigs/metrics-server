// A Riemann client for Go, featuring concurrency, sending events and state updates, queries
//
// Copyright (C) 2014 by Christopher Gilbert <christopher.john.gilbert@gmail.com>
package riemanngo

import (
	"github.com/riemann/riemann-go-client/proto"
)

// Client is an interface to a generic client
type Client interface {
	Send(message *proto.Msg) (*proto.Msg, error)
	Connect(timeout int32) error
	Close() error
}

// IndexClient is an interface to a generic Client for index queries
type IndexClient interface {
	QueryIndex(q string) ([]Event, error)
}

// request encapsulates a request to send to the Riemann server
type request struct {
	message     *proto.Msg
	response_ch chan response
}

// response encapsulates a response from the Riemann server
type response struct {
	message *proto.Msg
	err     error
}

// Send an event using a client
func SendEvent(c Client, e *Event) (*proto.Msg, error) {
	epb, err := EventToProtocolBuffer(e)
	if err != nil {
		return nil, err
	}
	message := &proto.Msg{}
	message.Events = append(message.Events, epb)

	msg, err := c.Send(message)
	return msg, err
}

// Send multiple events using a client
func SendEvents(c Client, e *[]Event) (*proto.Msg, error) {
	var events []*proto.Event
	for _, elem := range *e {
		epb, err := EventToProtocolBuffer(&elem)
		if err != nil {
			return nil, err
		}
		events = append(events, epb)
	}
	message := &proto.Msg{}
	message.Events = events

	msg, err := c.Send(message)
	return msg, err
}
