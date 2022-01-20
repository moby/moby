fluent-logger-golang
====

[![Build Status](https://github.com/fluent/fluent-logger-golang/actions/workflows/ci.yaml/badge.svg?branch=master)](https://github.com/fluent/fluent-logger-golang/actions)
[![GoDoc](https://godoc.org/github.com/fluent/fluent-logger-golang/fluent?status.svg)](https://godoc.org/github.com/fluent/fluent-logger-golang/fluent)

## A structured event logger for Fluentd (Golang)

## How to install

```
go get github.com/fluent/fluent-logger-golang/fluent
```

## Usage

Install the package with `go get` and use `import` to include it in your project.

```
import "github.com/fluent/fluent-logger-golang/fluent"
```

## Example

```go
package main

import (
  "github.com/fluent/fluent-logger-golang/fluent"
  "fmt"
  //"time"
)

func main() {
  logger, err := fluent.New(fluent.Config{})
  if err != nil {
    fmt.Println(err)
  }
  defer logger.Close()
  tag := "myapp.access"
  var data = map[string]string{
    "foo":  "bar",
    "hoge": "hoge",
  }
  error := logger.Post(tag, data)
  // error := logger.PostWithTime(tag, time.Now(), data)
  if error != nil {
    panic(error)
  }
}
```

`data` must be a value like `map[string]literal`, `map[string]interface{}`, `struct` or [`msgp.Marshaler`](http://godoc.org/github.com/tinylib/msgp/msgp#Marshaler). Logger refers tags `msg` or `codec` of each fields of structs.

## Setting config values

```go
f := fluent.New(fluent.Config{FluentPort: 80, FluentHost: "example.com"})
```

### FluentNetwork

Specify the network protocol. The supported values are:

 * "tcp" (use `FluentHost` and `FluentPort`)
 * "tls" (use`FluentHost` and `FluentPort`)
 * "unix" (use `FluentSocketPath`)

The default is "tcp".

### FluentHost

Specify a hostname or IP address as a string for the destination of the "tcp" protocol.
The default is "127.0.0.1".

### FluentPort

Specify the TCP port of the destination. The default is 24224.

### FluentSocketPath

Specify the unix socket path when `FluentNetwork` is "unix".

### Timeout

Set the timeout value of `time.Duration` to connect to the destination.
The default is 3 seconds.

### WriteTimeout

Sets the timeout value of `time.Duration` for Write call of `logger.Post`.
Since the default is zero value, Write will not time out.

### BufferLimit

Sets the number of events buffered on the memory. Records will be stored in memory up to this number. If the buffer is full, the call to record logs will fail.
The default is 8192.

### RetryWait

Set the duration of the initial wait for the first retry, in milliseconds. The actual retry wait will be `r * 1.5^(N-1)` (r: this value, N: the number of retries).
The default is 500.

### MaxRetry

Sets the maximum number of retries. If the number of retries become larger than this value, the write/send operation will fail.
The default is 13.

### MaxRetryWait

The maximum duration of wait between retries, in milliseconds. If the calculated retry wait is larger than this value, the actual retry wait will be this value.
The default is 60,000 (60 seconds).

### TagPrefix

Sets the prefix string of the tag. Prefix will be appended with a dot `.`, like `ppp.tag` (ppp: the value of this parameter, tag: the tag string specified in a call).
The default is blank.

### Async

Enable asynchronous I/O (connect and write) for sending events to Fluentd.
The default is false.

### ForceStopAsyncSend

When Async is enabled, immediately discard the event queue on close() and return (instead of trying MaxRetry times for each event in the queue before returning)
The default is false.

### AsyncResultCallback

When Async is enabled, if this is callback is provided, it will be called on every write to Fluentd. The callback function
takes two arguments - a `[]byte` of the message that was to be sent and an `error`. If the `error` is not nil this means the 
delivery of the message was unsuccessful.

### SubSecondPrecision

Enable time encoding as EventTime, which contains sub-second precision values. The messages encoded with this option can be received only by Fluentd v0.14 or later.
The default is false.

### MarshalAsJson

Enable Json data marshaling to send messages using Json format (instead of the standard MessagePack). It is supported by Fluentd `in_forward` plugin.
The default is false.

### RequestAck

Sets whether to request acknowledgment from Fluentd to increase the reliability
of the connection. The default is false.

### TlsInsecureSkipVerify

Skip verifying the server certificate. Useful for development and testing. The default is false.

## FAQ

### Does this logger support the features of Fluentd Forward Protocol v1?

"the features" includes heartbeat messages (for TCP keepalive), TLS transport and shared key authentication.

This logger doesn't support those features. Patches are welcome!

### Is it allowed to call `Fluent.Post()` after connection close?

Before v1.8.0, the Fluent logger silently reopened connections whenever
`Fluent.Post()` was called.

```go
logger, _ := fluent.New(fluent.Config{})
logger.Post(tag, data)
logger.Close()
logger.Post(tag, data)  /* reopen connection */
```

However, this behavior was confusing, in particular when multiple goroutines
were involved. Starting v1.8.0, the logger no longer accepts `Fluent.Post()`
after `Fluent.Close()`, and instead returns a "Logger already closed" error.

## Tests
```

go test
```
