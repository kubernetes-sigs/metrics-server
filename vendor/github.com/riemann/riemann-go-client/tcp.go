package riemanngo

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"time"

	pb "github.com/golang/protobuf/proto"
	"github.com/riemann/riemann-go-client/proto"
)

// TcpClient is a type that implements the Client interface
type TcpClient struct {
	addr         string
	conn         net.Conn
	requestQueue chan request
}

// NewTcpClient - Factory
func NewTcpClient(addr string) *TcpClient {
	t := &TcpClient{
		addr:         addr,
		requestQueue: make(chan request),
	}
	go t.runRequestQueue()
	return t
}

// connect the TcpClient
func (c *TcpClient) Connect(timeout int32) error {
	tcp, err := net.DialTimeout("tcp", c.addr, time.Second*time.Duration(timeout))
	if err != nil {
		return err
	}
	c.conn = tcp
	return nil
}

// TcpClient implementation of Send, queues a request to send a message to the server
func (t *TcpClient) Send(message *proto.Msg) (*proto.Msg, error) {
	response_ch := make(chan response)
	t.requestQueue <- request{message, response_ch}
	r := <-response_ch
	return r.message, r.err
}

// Close will close the TcpClient
func (t *TcpClient) Close() error {
	close(t.requestQueue)
	err := t.conn.Close()
	return err
}

// runRequestQueue services the TcpClient request queue
func (t *TcpClient) runRequestQueue() {
	for req := range t.requestQueue {
		message := req.message
		response_ch := req.response_ch

		msg, err := t.execRequest(message)

		response_ch <- response{msg, err}
	}
}

// execRequest will send a TCP message to Riemann
func (t *TcpClient) execRequest(message *proto.Msg) (*proto.Msg, error) {
	msg := &proto.Msg{}
	data, err := pb.Marshal(message)
	if err != nil {
		return msg, err
	}
	b := new(bytes.Buffer)
	if err = binary.Write(b, binary.BigEndian, uint32(len(data))); err != nil {
		return msg, err
	}
	// send the msg length
	if _, err = t.conn.Write(b.Bytes()); err != nil {
		return msg, err
	}
	// send the msg
	if _, err = t.conn.Write(data); err != nil {
		return msg, err
	}
	var header uint32
	if err = binary.Read(t.conn, binary.BigEndian, &header); err != nil {
		return msg, err
	}
	response := make([]byte, header)
	if err = readMessages(t.conn, response); err != nil {
		return msg, err
	}
	if err = pb.Unmarshal(response, msg); err != nil {
		return msg, err
	}
	return msg, nil
}

// readMessages will read Riemann messages from the TCP connection
func readMessages(r io.Reader, p []byte) error {
	for len(p) > 0 {
		n, err := r.Read(p)
		p = p[n:]
		if err != nil {
			return err
		}
	}
	return nil
}

// Query the server for events using the client
func (c *TcpClient) QueryIndex(q string) ([]Event, error) {
	query := &proto.Query{}
	query.String_ = pb.String(q)

	message := &proto.Msg{}
	message.Query = query

	response, err := c.Send(message)
	if err != nil {
		return nil, err
	}

	return ProtocolBuffersToEvents(response.GetEvents()), nil
}
