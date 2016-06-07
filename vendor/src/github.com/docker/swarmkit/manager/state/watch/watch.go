package watch

import "github.com/docker/go-events"

// Queue is the structure used to publish events and watch for them.
type Queue struct {
	broadcast *events.Broadcaster
}

// NewQueue creates a new publish/subscribe queue which supports watchers.
// The channels that it will create for subscriptions will have the buffer
// size specified by buffer.
func NewQueue(buffer int) *Queue {
	return &Queue{
		broadcast: events.NewBroadcaster(),
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
	sink := events.Sink(events.NewQueue(ch))

	if matcher != nil {
		sink = events.NewFilter(sink, matcher)
	}

	q.broadcast.Add(sink)
	return ch.C, func() {
		q.broadcast.Remove(sink)
		ch.Close()
		sink.Close()
	}
}

// Publish adds an item to the queue.
func (q *Queue) Publish(item events.Event) {
	q.broadcast.Write(item)
}
