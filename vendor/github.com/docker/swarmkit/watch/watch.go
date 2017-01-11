package watch

import (
	"sync"

	"github.com/docker/go-events"
)

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

// Queue is the structure used to publish events and watch for them.
type Queue struct {
	mu          sync.Mutex
	broadcast   *events.Broadcaster
	cancelFuncs map[*events.Channel]func()
}

// NewQueue creates a new publish/subscribe queue which supports watchers.
// The channels that it will create for subscriptions will have the buffer
// size specified by buffer.
func NewQueue() *Queue {
	return &Queue{
		broadcast:   events.NewBroadcaster(),
		cancelFuncs: make(map[*events.Channel]func()),
	}
}

// Watch returns a channel which will receive all items published to the
// queue from this point, until cancel is called.
func (q *Queue) Watch() (eventq chan events.Event, cancel func()) {
	return q.CallbackWatch(nil)
}

// CallbackWatch returns a channel which will receive all events published to
// the queue from this point that pass the check in the provided callback
// function. The returned cancel function will stop the flow of events and
// close the channel.
func (q *Queue) CallbackWatch(matcher events.Matcher) (eventq chan events.Event, cancel func()) {
	ch := events.NewChannel(0)
	sink := events.Sink(events.NewQueue(dropErrClosed{sink: ch}))

	if matcher != nil {
		sink = events.NewFilter(sink, matcher)
	}

	q.broadcast.Add(sink)

	cancelFunc := func() {
		q.broadcast.Remove(sink)
		ch.Close()
		sink.Close()
	}

	q.mu.Lock()
	q.cancelFuncs[ch] = cancelFunc
	q.mu.Unlock()
	return ch.C, func() {
		q.mu.Lock()
		cancelFunc := q.cancelFuncs[ch]
		delete(q.cancelFuncs, ch)
		q.mu.Unlock()

		if cancelFunc != nil {
			cancelFunc()
		}
	}
}

// Publish adds an item to the queue.
func (q *Queue) Publish(item events.Event) {
	q.broadcast.Write(item)
}

// Close closes the queue and frees the associated resources.
func (q *Queue) Close() error {
	// Make sure all watchers have been closed to avoid a deadlock when
	// closing the broadcaster.
	q.mu.Lock()
	for _, cancelFunc := range q.cancelFuncs {
		cancelFunc()
	}
	q.cancelFuncs = make(map[*events.Channel]func())
	q.mu.Unlock()

	return q.broadcast.Close()
}
