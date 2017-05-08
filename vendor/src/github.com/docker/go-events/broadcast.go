package events

import "github.com/Sirupsen/logrus"

// Broadcaster sends events to multiple, reliable Sinks. The goal of this
// component is to dispatch events to configured endpoints. Reliability can be
// provided by wrapping incoming sinks.
type Broadcaster struct {
	sinks   []Sink
	events  chan Event
	adds    chan configureRequest
	removes chan configureRequest
	closed  chan chan struct{}
}

// NewBroadcaster appends one or more sinks to the list of sinks. The
// broadcaster behavior will be affected by the properties of the sink.
// Generally, the sink should accept all messages and deal with reliability on
// its own. Use of EventQueue and RetryingSink should be used here.
func NewBroadcaster(sinks ...Sink) *Broadcaster {
	b := Broadcaster{
		sinks:   sinks,
		events:  make(chan Event),
		adds:    make(chan configureRequest),
		removes: make(chan configureRequest),
		closed:  make(chan chan struct{}),
	}

	// Start the broadcaster
	go b.run()

	return &b
}

// Write accepts an event to be dispatched to all sinks. This method will never
// fail and should never block (hopefully!). The caller cedes the memory to the
// broadcaster and should not modify it after calling write.
func (b *Broadcaster) Write(event Event) error {
	select {
	case b.events <- event:
	case <-b.closed:
		return ErrSinkClosed
	}
	return nil
}

// Add the sink to the broadcaster.
//
// The provided sink must be comparable with equality. Typically, this just
// works with a regular pointer type.
func (b *Broadcaster) Add(sink Sink) error {
	return b.configure(b.adds, sink)
}

// Remove the provided sink.
func (b *Broadcaster) Remove(sink Sink) error {
	return b.configure(b.removes, sink)
}

type configureRequest struct {
	sink     Sink
	response chan error
}

func (b *Broadcaster) configure(ch chan configureRequest, sink Sink) error {
	response := make(chan error, 1)

	for {
		select {
		case ch <- configureRequest{
			sink:     sink,
			response: response}:
			ch = nil
		case err := <-response:
			return err
		case <-b.closed:
			return ErrSinkClosed
		}
	}
}

// Close the broadcaster, ensuring that all messages are flushed to the
// underlying sink before returning.
func (b *Broadcaster) Close() error {
	select {
	case <-b.closed:
		// already closed
		return ErrSinkClosed
	default:
		// do a little chan handoff dance to synchronize closing
		closed := make(chan struct{})
		b.closed <- closed
		close(b.closed)
		<-closed
		return nil
	}
}

// run is the main broadcast loop, started when the broadcaster is created.
// Under normal conditions, it waits for events on the event channel. After
// Close is called, this goroutine will exit.
func (b *Broadcaster) run() {
	remove := func(target Sink) {
		for i, sink := range b.sinks {
			if sink == target {
				b.sinks = append(b.sinks[:i], b.sinks[i+1:]...)
				break
			}
		}
	}

	for {
		select {
		case event := <-b.events:
			for _, sink := range b.sinks {
				if err := sink.Write(event); err != nil {
					if err == ErrSinkClosed {
						// remove closed sinks
						remove(sink)
						continue
					}
					logrus.WithField("event", event).WithField("events.sink", sink).WithError(err).
						Errorf("broadcaster: dropping event")
				}
			}
		case request := <-b.adds:
			// while we have to iterate for add/remove, common iteration for
			// send is faster against slice.

			var found bool
			for _, sink := range b.sinks {
				if request.sink == sink {
					found = true
					break
				}
			}

			if !found {
				b.sinks = append(b.sinks, request.sink)
			}
			// b.sinks[request.sink] = struct{}{}
			request.response <- nil
		case request := <-b.removes:
			remove(request.sink)
			request.response <- nil
		case closing := <-b.closed:
			// close all the underlying sinks
			for _, sink := range b.sinks {
				if err := sink.Close(); err != nil && err != ErrSinkClosed {
					logrus.WithField("events.sink", sink).WithError(err).
						Errorf("broadcaster: closing sink failed")
				}
			}
			closing <- struct{}{}
			return
		}
	}
}
