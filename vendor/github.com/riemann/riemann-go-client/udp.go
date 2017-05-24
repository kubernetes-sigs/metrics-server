package riemanngo

import (
	"fmt"
	"net"
	"time"

	pb "github.com/golang/protobuf/proto"
	"github.com/riemann/riemann-go-client/proto"
)

// UdpClient is a type that implements the Client interface
type UdpClient struct {
	addr         string
	conn         net.Conn
	requestQueue chan request
}

// MAX_UDP_SIZE is the maximum allowed size of a UDP packet before automatically failing the send
const MAX_UDP_SIZE = 16384

// NewUdpClient - Factory
func NewUdpClient(addr string) *UdpClient {
	t := &UdpClient{
		addr:         addr,
		requestQueue: make(chan request),
	}
	go t.runRequestQueue()
	return t
}

func (c *UdpClient) Connect(timeout int32) error {
	udp, err := net.DialTimeout("udp", c.addr, time.Second*time.Duration(timeout))
	if err != nil {
		return err
	}
	c.conn = udp
	return nil
}

func (t *UdpClient) Send(message *proto.Msg) (*proto.Msg, error) {
	response_ch := make(chan response)
	t.requestQueue <- request{message, response_ch}
	r := <-response_ch
	return r.message, r.err
}

// Close will close the UdpClient
func (t *UdpClient) Close() error {
	close(t.requestQueue)
	err := t.conn.Close()
	return err
}

// runRequestQueue services the UdpClient request queue
func (t *UdpClient) runRequestQueue() {
	for req := range t.requestQueue {
		message := req.message
		response_ch := req.response_ch

		msg, err := t.execRequest(message)

		response_ch <- response{msg, err}
	}
}

// execRequest will send a UDP message to Riemann
func (t *UdpClient) execRequest(message *proto.Msg) (*proto.Msg, error) {
	data, err := pb.Marshal(message)
	if err != nil {
		return nil, err
	}
	if len(data) > MAX_UDP_SIZE {
		return nil, fmt.Errorf("unable to send message, too large for udp")
	}
	if _, err = t.conn.Write(data); err != nil {
		return nil, err
	}
	return nil, nil
}
