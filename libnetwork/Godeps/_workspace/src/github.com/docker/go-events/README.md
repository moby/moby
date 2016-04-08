# Docker Events Package

[![GoDoc](https://godoc.org/github.com/docker/go-events?status.svg)](https://godoc.org/github.com/docker/go-events)
[![Circle CI](https://circleci.com/gh/docker/go-events.svg?style=shield)](https://circleci.com/gh/docker/go-events)

The Docker `events` package implements a composable event distribution package
for Go.

Originally created to implement the [notifications in Docker Registry
2](https://github.com/docker/distribution/blob/master/docs/notifications.md),
we've found the pattern to be useful in other applications. This package is
most of the same code with slightly updated interfaces. Much of the internals
have been made available.

## Usage

The `events` package centers around a `Sink` type.  Events are written with
calls to `Sink.Write(event Event)`. Sinks can be wired up in various
configurations to achieve interesting behavior.

The canonical example is that employed by the
[docker/distribution/notifications](https://godoc.org/github.com/docker/distribution/notifications)
package. Let's say we have a type `httpSink` where we'd like to queue
notifications. As a rule, it should send a single http request and return an
error if it fails:

```go
func (h *httpSink) Write(event Event) error {
	p, err := json.Marshal(event)
	if err != nil {
		return err
	}
	body := bytes.NewReader(p)
	resp, err := h.client.Post(h.url, "application/json", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.Status != 200 {
		return errors.New("unexpected status")
	}

	return nil
}

// implement (*httpSink).Close()
```

With just that, we can start using components from this package. One can call
`(*httpSink).Write` to send events as the body of a post request to a
configured URL.

### Retries

HTTP can be unreliable. The first feature we'd like is to have some retry:

```go
hs := newHTTPSink(/*...*/)
retry := NewRetryingSink(hs, NewBreaker(5, time.Second))
```

We now have a sink that will retry events against the `httpSink` until they
succeed. The retry will backoff for one second after 5 consecutive failures
using the breaker strategy.

### Queues

This isn't quite enough. We we want a sink that doesn't block while we are
waiting for events to be sent. Let's add a `Queue`:

```go
queue := NewQueue(retry)
```

Now, we have an unbounded queue that will work through all events sent with
`(*Queue).Write`. Events can be added asynchronously to the queue without
blocking the current execution path. This is ideal for use in an http request.

### Broadcast

It usually turns out that you want to send to more than one listener. We can
use `Broadcaster` to support this:

```go
var broadcast = NewBroadcaster() // make it available somewhere in your application.
broadcast.Add(queue) // add your queue!
broadcast.Add(queue2) // and another!
```

With the above, we can now call `broadcast.Write` in our http handlers and have
all the events distributed to each queue. Because the events are queued, not
listener blocks another.

### Extending

For the most part, the above is sufficient for a lot of applications. However,
extending the above functionality can be done implementing your own `Sink`. The
behavior and semantics of the sink can be completely dependent on the
application requirements. The interface is provided below for reference:

```go
type Sink {
	Write(Event) error
	Close() error
}
```

Application behavior can be controlled by how `Write` behaves. The examples
above are designed to queue the message and return as quickly as possible.
Other implementations may block until the event is committed to durable
storage.
