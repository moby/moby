package queue

import (
	"container/list"
	"fmt"
	"sync"

	"github.com/docker/go-events"
	"github.com/sirupsen/logrus"
)

// ErrQueueFull is returned by a Write operation when that Write causes the
// queue to reach its size limit.
var ErrQueueFull = fmt.Errorf("queue closed due to size limit")

// LimitQueue accepts all messages into a queue for asynchronous consumption by
// a sink until an upper limit of messages is reached. When that limit is
// reached, the entire Queue is Closed. It is thread safe but the
// sink must be reliable or events will be dropped.
// If a size of 0 is provided, the LimitQueue is considered limitless.
type LimitQueue struct {
	dst        events.Sink
	events     *list.List
	limit      uint64
	cond       *sync.Cond
	mu         sync.Mutex
	closed     bool
	full       chan struct{}
	fullClosed bool
}

// NewLimitQueue returns a queue to the provided Sink dst.
func NewLimitQueue(dst events.Sink, limit uint64) *LimitQueue {
	eq := LimitQueue{
		dst:    dst,
		events: list.New(),
		limit:  limit,
		full:   make(chan struct{}),
	}

	eq.cond = sync.NewCond(&eq.mu)
	go eq.run()
	return &eq
}

// Write accepts the events into the queue, only failing if the queue has
// been closed or has reached its size limit.
func (eq *LimitQueue) Write(event events.Event) error {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	if eq.closed {
		return events.ErrSinkClosed
	}

	if eq.limit > 0 && uint64(eq.events.Len()) >= eq.limit {
		// If the limit has been reached, don't write the event to the queue,
		// and close the Full channel. This notifies listeners that the queue
		// is now full, but the sink is still permitted to consume events. It's
		// the responsibility of the listener to decide whether they want to
		// live with dropped events or whether they want to Close() the
		// LimitQueue
		if !eq.fullClosed {
			eq.fullClosed = true
			close(eq.full)
		}
		return ErrQueueFull
	}

	eq.events.PushBack(event)
	eq.cond.Signal() // signal waiters

	return nil
}

// Full returns a channel that is closed when the queue becomes full for the
// first time.
func (eq *LimitQueue) Full() chan struct{} {
	return eq.full
}

// Close shuts down the event queue, flushing all events
func (eq *LimitQueue) Close() error {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	if eq.closed {
		return nil
	}

	// set the closed flag
	eq.closed = true
	eq.cond.Signal() // signal flushes queue
	eq.cond.Wait()   // wait for signal from last flush
	return eq.dst.Close()
}

// run is the main goroutine to flush events to the target sink.
func (eq *LimitQueue) run() {
	for {
		event := eq.next()

		if event == nil {
			return // nil block means event queue is closed.
		}

		if err := eq.dst.Write(event); err != nil {
			// TODO(aaronl): Dropping events could be bad depending
			// on the application. We should have a way of
			// communicating this condition. However, logging
			// at a log level above debug may not be appropriate.
			// Eventually, go-events should not use logrus at all,
			// and should bubble up conditions like this through
			// error values.
			logrus.WithFields(logrus.Fields{
				"event": event,
				"sink":  eq.dst,
			}).WithError(err).Debug("eventqueue: dropped event")
		}
	}
}

// Len returns the number of items that are currently stored in the queue and
// not consumed by its sink.
func (eq *LimitQueue) Len() int {
	eq.mu.Lock()
	defer eq.mu.Unlock()
	return eq.events.Len()
}

func (eq *LimitQueue) String() string {
	eq.mu.Lock()
	defer eq.mu.Unlock()
	return fmt.Sprintf("%v", eq.events)
}

// next encompasses the critical section of the run loop. When the queue is
// empty, it will block on the condition. If new data arrives, it will wake
// and return a block. When closed, a nil slice will be returned.
func (eq *LimitQueue) next() events.Event {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	for eq.events.Len() < 1 {
		if eq.closed {
			eq.cond.Broadcast()
			return nil
		}

		eq.cond.Wait()
	}

	front := eq.events.Front()
	block := front.Value.(events.Event)
	eq.events.Remove(front)

	return block
}
