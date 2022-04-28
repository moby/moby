package watch

import (
	"fmt"
	"time"

	events "github.com/docker/go-events"
)

// ErrSinkTimeout is returned from the Write method when a sink times out.
var ErrSinkTimeout = fmt.Errorf("timeout exceeded, tearing down sink")

// timeoutSink is a sink that wraps another sink with a timeout. If the
// embedded sink fails to complete a Write operation within the specified
// timeout, the Write operation of the timeoutSink fails.
type timeoutSink struct {
	timeout time.Duration
	sink    events.Sink
}

func (s timeoutSink) Write(event events.Event) error {
	errChan := make(chan error)
	go func(c chan<- error) {
		c <- s.sink.Write(event)
	}(errChan)

	timer := time.NewTimer(s.timeout)
	select {
	case err := <-errChan:
		timer.Stop()
		return err
	case <-timer.C:
		s.sink.Close()
		return ErrSinkTimeout
	}
}

func (s timeoutSink) Close() error {
	return s.sink.Close()
}

// dropErrClosed is a sink that suppresses ErrSinkClosed from Write, to avoid
// debug log messages that may be confusing. It is possible that the queue
// will try to write an event to its destination channel while the queue is
// being removed from the broadcaster. Since the channel is closed before the
// queue, there is a narrow window when this is possible. In some event-based
// dropping events when a sink is removed from a broadcaster is a problem, but
// for the usage in this watch package that's the expected behavior.
type dropErrClosed struct {
	sink events.Sink
}

func (s dropErrClosed) Write(event events.Event) error {
	err := s.sink.Write(event)
	if err == events.ErrSinkClosed {
		return nil
	}
	return err
}

func (s dropErrClosed) Close() error {
	return s.sink.Close()
}

// dropErrClosedChanGen is a ChannelSinkGenerator for dropErrClosed sinks wrapping
// unbuffered channels.
type dropErrClosedChanGen struct{}

func (s *dropErrClosedChanGen) NewChannelSink() (events.Sink, *events.Channel) {
	ch := events.NewChannel(0)
	return dropErrClosed{sink: ch}, ch
}

// TimeoutDropErrChanGen is a ChannelSinkGenerator that creates a channel,
// wrapped by the dropErrClosed sink and a timeout.
type TimeoutDropErrChanGen struct {
	timeout time.Duration
}

// NewChannelSink creates a new sink chain of timeoutSink->dropErrClosed->Channel
func (s *TimeoutDropErrChanGen) NewChannelSink() (events.Sink, *events.Channel) {
	ch := events.NewChannel(0)
	return timeoutSink{
		timeout: s.timeout,
		sink: dropErrClosed{
			sink: ch,
		},
	}, ch
}

// NewTimeoutDropErrSinkGen returns a generator of timeoutSinks wrapping dropErrClosed
// sinks, wrapping unbuffered channel sinks.
func NewTimeoutDropErrSinkGen(timeout time.Duration) ChannelSinkGenerator {
	return &TimeoutDropErrChanGen{timeout: timeout}
}
