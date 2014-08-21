package events

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/utils"
)

const eventsLimit = 64

type listener chan<- *utils.JSONMessage

type Events struct {
	mu          sync.RWMutex
	events      []*utils.JSONMessage
	subscribers []listener
}

func New() *Events {
	return &Events{
		events: make([]*utils.JSONMessage, 0, eventsLimit),
	}
}

// Install installs events public api in docker engine
func (e *Events) Install(eng *engine.Engine) error {
	// Here you should describe public interface
	jobs := map[string]engine.Handler{
		"events":            e.Get,
		"log":               e.Log,
		"subscribers_count": e.SubscribersCount,
	}
	for name, job := range jobs {
		if err := eng.Register(name, job); err != nil {
			return err
		}
	}
	return nil
}

func (e *Events) Get(job *engine.Job) engine.Status {
	var (
		since   = job.GetenvInt64("since")
		until   = job.GetenvInt64("until")
		timeout = time.NewTimer(time.Unix(until, 0).Sub(time.Now()))
	)

	// If no until, disable timeout
	if until == 0 {
		timeout.Stop()
	}

	listener := make(chan *utils.JSONMessage)
	e.subscribe(listener)
	defer e.unsubscribe(listener)

	job.Stdout.Write(nil)

	// Resend every event in the [since, until] time interval.
	if since != 0 {
		if err := e.writeCurrent(job, since, until); err != nil {
			return job.Error(err)
		}
	}

	for {
		select {
		case event, ok := <-listener:
			if !ok {
				return engine.StatusOK
			}
			if err := writeEvent(job, event); err != nil {
				return job.Error(err)
			}
		case <-timeout.C:
			return engine.StatusOK
		}
	}
}

func (e *Events) Log(job *engine.Job) engine.Status {
	if len(job.Args) != 3 {
		return job.Errorf("usage: %s ACTION ID FROM", job.Name)
	}
	// not waiting for receivers
	go e.log(job.Args[0], job.Args[1], job.Args[2])
	return engine.StatusOK
}

func (e *Events) SubscribersCount(job *engine.Job) engine.Status {
	ret := &engine.Env{}
	ret.SetInt("count", e.subscribersCount())
	ret.WriteTo(job.Stdout)
	return engine.StatusOK
}

func writeEvent(job *engine.Job, event *utils.JSONMessage) error {
	// When sending an event JSON serialization errors are ignored, but all
	// other errors lead to the eviction of the listener.
	if b, err := json.Marshal(event); err == nil {
		if _, err = job.Stdout.Write(b); err != nil {
			return err
		}
	}
	return nil
}

func (e *Events) writeCurrent(job *engine.Job, since, until int64) error {
	e.mu.RLock()
	for _, event := range e.events {
		if event.Time >= since && (event.Time <= until || until == 0) {
			if err := writeEvent(job, event); err != nil {
				e.mu.RUnlock()
				return err
			}
		}
	}
	e.mu.RUnlock()
	return nil
}

func (e *Events) subscribersCount() int {
	e.mu.RLock()
	c := len(e.subscribers)
	e.mu.RUnlock()
	return c
}

func (e *Events) log(action, id, from string) {
	e.mu.Lock()
	now := time.Now().UTC().Unix()
	jm := &utils.JSONMessage{Status: action, ID: id, From: from, Time: now}
	if len(e.events) == cap(e.events) {
		// discard oldest event
		copy(e.events, e.events[1:])
		e.events[len(e.events)-1] = jm
	} else {
		e.events = append(e.events, jm)
	}
	for _, s := range e.subscribers {
		// We give each subscriber a 100ms time window to receive the event,
		// after which we move to the next.
		select {
		case s <- jm:
		case <-time.After(100 * time.Millisecond):
		}
	}
	e.mu.Unlock()
}

func (e *Events) subscribe(l listener) {
	e.mu.Lock()
	e.subscribers = append(e.subscribers, l)
	e.mu.Unlock()
}

// unsubscribe closes and removes the specified listener from the list of
// previously registed ones.
// It returns a boolean value indicating if the listener was successfully
// found, closed and unregistered.
func (e *Events) unsubscribe(l listener) bool {
	e.mu.Lock()
	for i, subscriber := range e.subscribers {
		if subscriber == l {
			close(l)
			e.subscribers = append(e.subscribers[:i], e.subscribers[i+1:]...)
			e.mu.Unlock()
			return true
		}
	}
	e.mu.Unlock()
	return false
}
