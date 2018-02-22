package watch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/docker/go-events"
	"github.com/docker/swarmkit/watch/queue"
)

// ChannelSinkGenerator is a constructor of sinks that eventually lead to a
// channel.
type ChannelSinkGenerator interface {
	NewChannelSink() (events.Sink, *events.Channel)
}

// Queue is the structure used to publish events and watch for them.
type Queue struct {
	sinkGen ChannelSinkGenerator
	// limit is the max number of items to be held in memory for a watcher
	limit       uint64
	mu          sync.Mutex
	broadcast   *events.Broadcaster
	cancelFuncs map[events.Sink]func()

	// closeOutChan indicates whether the watchers' channels should be closed
	// when a watcher queue reaches its limit or when the Close method of the
	// sink is called.
	closeOutChan bool
}

// NewQueue creates a new publish/subscribe queue which supports watchers.
// The channels that it will create for subscriptions will have the buffer
// size specified by buffer.
func NewQueue(options ...func(*Queue) error) *Queue {
	// Create a queue with the default values
	q := &Queue{
		sinkGen:      &dropErrClosedChanGen{},
		broadcast:    events.NewBroadcaster(),
		cancelFuncs:  make(map[events.Sink]func()),
		limit:        0,
		closeOutChan: false,
	}

	for _, option := range options {
		err := option(q)
		if err != nil {
			panic(fmt.Sprintf("Failed to apply options to queue: %s", err))
		}
	}

	return q
}

// WithTimeout returns a functional option for a queue that sets a write timeout
func WithTimeout(timeout time.Duration) func(*Queue) error {
	return func(q *Queue) error {
		q.sinkGen = NewTimeoutDropErrSinkGen(timeout)
		return nil
	}
}

// WithCloseOutChan returns a functional option for a queue whose watcher
// channel is closed when no more events are expected to be sent to the watcher.
func WithCloseOutChan() func(*Queue) error {
	return func(q *Queue) error {
		q.closeOutChan = true
		return nil
	}
}

// WithLimit returns a functional option for a queue with a max size limit.
func WithLimit(limit uint64) func(*Queue) error {
	return func(q *Queue) error {
		q.limit = limit
		return nil
	}
}

// Watch returns a channel which will receive all items published to the
// queue from this point, until cancel is called.
func (q *Queue) Watch() (eventq chan events.Event, cancel func()) {
	return q.CallbackWatch(nil)
}

// WatchContext returns a channel where all items published to the queue will
// be received. The channel will be closed when the provided context is
// cancelled.
func (q *Queue) WatchContext(ctx context.Context) (eventq chan events.Event) {
	return q.CallbackWatchContext(ctx, nil)
}

// CallbackWatch returns a channel which will receive all events published to
// the queue from this point that pass the check in the provided callback
// function. The returned cancel function will stop the flow of events and
// close the channel.
func (q *Queue) CallbackWatch(matcher events.Matcher) (eventq chan events.Event, cancel func()) {
	chanSink, ch := q.sinkGen.NewChannelSink()
	lq := queue.NewLimitQueue(chanSink, q.limit)
	sink := events.Sink(lq)

	if matcher != nil {
		sink = events.NewFilter(sink, matcher)
	}

	q.broadcast.Add(sink)

	cancelFunc := func() {
		q.broadcast.Remove(sink)
		ch.Close()
		sink.Close()
	}

	externalCancelFunc := func() {
		q.mu.Lock()
		cancelFunc := q.cancelFuncs[sink]
		delete(q.cancelFuncs, sink)
		q.mu.Unlock()

		if cancelFunc != nil {
			cancelFunc()
		}
	}

	q.mu.Lock()
	q.cancelFuncs[sink] = cancelFunc
	q.mu.Unlock()

	// If the output channel shouldn't be closed and the queue is limitless,
	// there's no need for an additional goroutine.
	if !q.closeOutChan && q.limit == 0 {
		return ch.C, externalCancelFunc
	}

	outChan := make(chan events.Event)
	go func() {
		for {
			select {
			case <-ch.Done():
				// Close the output channel if the ChannelSink is Done for any
				// reason. This can happen if the cancelFunc is called
				// externally or if it has been closed by a wrapper sink, such
				// as the TimeoutSink.
				if q.closeOutChan {
					close(outChan)
				}
				externalCancelFunc()
				return
			case <-lq.Full():
				// Close the output channel and tear down the Queue if the
				// LimitQueue becomes full.
				if q.closeOutChan {
					close(outChan)
				}
				externalCancelFunc()
				return
			case event := <-ch.C:
				outChan <- event
			}
		}
	}()

	return outChan, externalCancelFunc
}

// CallbackWatchContext returns a channel where all items published to the queue will
// be received. The channel will be closed when the provided context is
// cancelled.
func (q *Queue) CallbackWatchContext(ctx context.Context, matcher events.Matcher) (eventq chan events.Event) {
	c, cancel := q.CallbackWatch(matcher)
	go func() {
		<-ctx.Done()
		cancel()
	}()
	return c
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
	q.cancelFuncs = make(map[events.Sink]func())
	q.mu.Unlock()

	return q.broadcast.Close()
}
