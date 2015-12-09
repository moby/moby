package events

import (
	"sync"
	"time"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/pubsub"
)

const (
	eventsLimit = 64
	bufferSize  = 1024
)

// Events is pubsub channel for *jsonmessage.JSONMessage
type Events struct {
	mu     sync.Mutex
	events []*jsonmessage.JSONMessage
	pub    *pubsub.Publisher
}

// New returns new *Events instance
func New() *Events {
	return &Events{
		events: make([]*jsonmessage.JSONMessage, 0, eventsLimit),
		pub:    pubsub.NewPublisher(100*time.Millisecond, bufferSize),
	}
}

// Subscribe adds new listener to events, returns slice of 64 stored
// last events, a channel in which you can expect new events (in form
// of interface{}, so you need type assertion), and a function to call
// to stop the stream of events.
func (e *Events) Subscribe() ([]*jsonmessage.JSONMessage, chan interface{}, func()) {
	e.mu.Lock()
	current := make([]*jsonmessage.JSONMessage, len(e.events))
	copy(current, e.events)
	l := e.pub.Subscribe()
	e.mu.Unlock()

	cancel := func() {
		e.Evict(l)
	}
	return current, l, cancel
}

// SubscribeTopic adds new listener to events, returns slice of 64 stored
// last events, a channel in which you can expect new events (in form
// of interface{}, so you need type assertion).
func (e *Events) SubscribeTopic(since, sinceNano int64, ef *Filter) ([]*jsonmessage.JSONMessage, chan interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var buffered []*jsonmessage.JSONMessage
	topic := func(m interface{}) bool {
		return ef.Include(m.(*jsonmessage.JSONMessage))
	}

	if since != -1 {
		for i := len(e.events) - 1; i >= 0; i-- {
			ev := e.events[i]
			if ev.Time < since || ((ev.Time == since) && (ev.TimeNano < sinceNano)) {
				break
			}
			if ef.filter.Len() == 0 || topic(ev) {
				buffered = append([]*jsonmessage.JSONMessage{ev}, buffered...)
			}
		}
	}

	var ch chan interface{}
	if ef.filter.Len() > 0 {
		ch = e.pub.SubscribeTopic(topic)
	} else {
		// Subscribe to all events if there are no filters
		ch = e.pub.Subscribe()
	}

	return buffered, ch
}

// Evict evicts listener from pubsub
func (e *Events) Evict(l chan interface{}) {
	e.pub.Evict(l)
}

// Log broadcasts event to listeners. Each listener has 100 millisecond for
// receiving event or it will be skipped.
func (e *Events) Log(action, id, from string) {
	now := time.Now().UTC()
	jm := &jsonmessage.JSONMessage{Status: action, ID: id, From: from, Time: now.Unix(), TimeNano: now.UnixNano()}
	e.mu.Lock()
	if len(e.events) == cap(e.events) {
		// discard oldest event
		copy(e.events, e.events[1:])
		e.events[len(e.events)-1] = jm
	} else {
		e.events = append(e.events, jm)
	}
	e.mu.Unlock()
	e.pub.Publish(jm)
}

// SubscribersCount returns number of event listeners
func (e *Events) SubscribersCount() int {
	return e.pub.Len()
}
