# Riemann client (Golang)

## Introduction

A Go client library for [Riemann](https://github.com/riemann/riemann).

Features:
* Idiomatic concurrency
* Sending events, queries.
* Support TCP, UDP, TLS clients.

This client is a fork of Goryman, a Riemann go client written by Christopher Gilbert. Thanks and full credit to him!

The initial [Goryman repository](https://github.com/bigdatadev/goryman) has been deleted. We've used [@rikatz's fork](https://github.com/rikatz/goryman/) to create this repository.

We've also renamed the client to `riemanngo` instead of `goryman` to make its purpose clearer.

## Build Status

[![Build Status](https://travis-ci.org/riemann/riemann-go-client.svg?branch=master)](https://travis-ci.org/riemann/riemann-go-client)

## Installation

To install the package for use in your own programs:

```
go get github.com/riemann/riemann-go-client
```

If you're a developer, Riemann uses [Google Protocol Buffers](https://github.com/golang/protobuf), so make sure that's installed and available on your `PATH`.

```
go get github.com/golang/protobuf/{proto,protoc-gen-go}
```

## Getting Started

First, we'll need to import the library:

```go
import (
    "github.com/riemann/riemann-go-client/"
)
```

Next, we'll need to establish a new client. The parameter is the connection timeout duration. You can use a TCP client:


```go
c := riemanngo.NewTcpClient("127.0.0.1:5555")
err := c.Connect(5)
if err != nil {
    panic(err)
}
```

Or a UDP client:

```go
c := riemanngo.NewUdpClient("127.0.0.1:5555")
err := c.Connect(5)
if err != nil {
    panic(err)
}
```

You can also create a TLS client.
The second parameter is the path to your client certificate, the third parameter the path to your client key. The last parameter allows you to create an insecure connection (insecure certificate check).
You can find more informations about how to configure TLS in Riemann [here](http://riemann.io/howto.html#securing-traffic-using-tls).

```go
c := riemanngo.NewTlsClient("127.0.0.1:5554", "/path/to/cert.pem", "/path/to/key.key", true)
err := c.Connect(5)
if err != nil {
    panic(err)
}
```

Don't forget to close the client connection when you're done:

```go
defer c.Close()
```

Sending events is easy ([list of valid event properties](http://riemann.io/concepts.html)):

```go
result, err := riemanngo.SendEvent(c, &riemanngo.Event{
		Service: "hello",
		Metric:  100,
		Tags: []string{"riemann ftw"},
	})
```

The host name and time in events will automatically be replaced with the hostname of the server and the current time if none is specified.

You can also send batch of events:

```go
events = []riemanngo.Event {
    riemanngo.Event{
        Service: "hello",
        Metric:  100,
        Tags: []string{"hello"},
    },
riemanngo.Event{
        Service: "goodbye",
        Metric:  200,
        Tags: []string{"goodbye"},
    },
}
```

You can also query the Riemann index (using the TCP or TLS client):

```go
events, err := c.QueryIndex("service = \"hello\"")
if err != nil {
    panic(err)
}
```

## Tests

You can lauch tests using

```
make test
```

and integration tests using:

```
make integ-test
```

This command will download Riemann, start it, launch integration tests, and kill it.

You can also use:

```
go test -tags=integration
```

if you already have a Riemann instance listening on localhost

## Copyright

See [LICENSE](LICENSE) document
