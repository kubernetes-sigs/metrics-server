graphite-golang
===============

This is a lightweight Graphite API client written in Go that implements Carbon
submission functionality. I wrote this a long time ago as a dependency for a side
project a long time ago. You shouldn't rely on this for any production use-case.

## Installation

Use `go-get` to install graphite-golang
```
go get github.com/marpaia/graphite-golang
```

## External dependencies

This project has no external dependencies other than the Go standard library.

## Documentation

Like most every other Golang project, this projects documentation can be found
on godoc at [godoc.org/github.com/marpaia/graphite-golang](http://godoc.org/github.com/marpaia/graphite-golang).

## Examples

```go
package mylib

import (
    "github.com/marpaia/graphite-golang"
    "log"
)

func init() {

    // load your configuration file / mechanism
    config := newConfig()

    // try to connect a graphite server
    if config.GraphiteEnabled {
        Graphite, err = graphite.NewGraphite(config.Graphite.Host, config.Graphite.Port)
    } else {
        Graphite = graphite.NewGraphiteNop(config.Graphite.Host, config.Graphite.Port)
    }
    // if you couldn't connect to graphite, use a nop
    if err != nil {
        Graphite = graphite.NewGraphiteNop(config.Graphite.Host, config.Graphite.Port)
    }

    log.Printf("Loaded Graphite connection: %#v", Graphite)
    Graphite.SimpleSend("stats.graphite_loaded", "1")
}

func doWork() {
    // this will work just fine, regardless of if you're working with a graphite
    // nop or not
    Graphite.SimpleSend("stats.doing_work", "1")
}
```
