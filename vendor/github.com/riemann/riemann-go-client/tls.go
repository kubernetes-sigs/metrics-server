package riemanngo

import (
	"bytes"
	"encoding/binary"
	"crypto/tls"
	"io/ioutil"
	"crypto/x509"
	"net"
	"time"
	"strings"

	pb "github.com/golang/protobuf/proto"
	"github.com/riemann/riemann-go-client/proto"
)

// TlsClient is a type that implements the Client interface
type TlsClient struct {
	addr          string
	tlsConfig     tls.Config
	conn          net.Conn
	requestQueue chan request
}

// NewTlsClient - Factory
func NewTlsClient(addr string, certPath string, keyPath string, insecure bool) (*TlsClient, error) {
	certFile, err := ioutil.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	clientCertPool := x509.NewCertPool()
	clientCertPool.AppendCertsFromPEM(certFile)

	config := tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      clientCertPool,
		InsecureSkipVerify: insecure}

	if !insecure {
		serverName := strings.Split(addr, ":")[0]
		config.ServerName = serverName
	}

	t := &TlsClient{
		addr:         addr,
		tlsConfig:    config,
		requestQueue: make(chan request),
	}
	go t.runRequestQueue()
	return t, nil
}

// connect the TlsClient
func (c *TlsClient) Connect(timeout int32) error {
	tcp, err := net.DialTimeout("tcp", c.addr, time.Second*time.Duration(timeout))
	if err != nil {
		return err
	}
	tlsConn := tls.Client(tcp, &c.tlsConfig)
	c.conn = tlsConn
	return nil
}


// TlsClient implementation of Send, queues a request to send a message to the server
func (t *TlsClient) Send(message *proto.Msg) (*proto.Msg, error) {
	response_ch := make(chan response)
	t.requestQueue <- request{message, response_ch}
	r := <-response_ch
	return r.message, r.err
}

// Close will close the TlsClient
func (t *TlsClient) Close() error {
	close(t.requestQueue)
	err := t.conn.Close()
	return err
}

// runRequestQueue services the TlsClient request queue
func (t *TlsClient) runRequestQueue() {
	for req := range t.requestQueue {
		message := req.message
		response_ch := req.response_ch

		msg, err := t.execRequest(message)

		response_ch <- response{msg, err}
	}
}

// execRequest will send a TCP message (using tls) to Riemann
func (t *TlsClient) execRequest(message *proto.Msg) (*proto.Msg, error) {
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

// Query the server for events using the client
func (c *TlsClient) QueryIndex(q string) ([]Event, error) {
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
