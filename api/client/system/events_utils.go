package system

import (
	"encoding/json"
	"io"
	"sync"

	"github.com/Sirupsen/logrus"
	eventtypes "github.com/docker/engine-api/types/events"
)

// EventHandler is abstract interface for user to customize
// own handle functions of each type of events
type EventHandler interface {
	Handle(action string, h func(eventtypes.Message))
	Watch(c <-chan eventtypes.Message)
}

// InitEventHandler initializes and returns an EventHandler
func InitEventHandler() EventHandler {
	return &eventHandler{handlers: make(map[string]func(eventtypes.Message))}
}

type eventHandler struct {
	handlers map[string]func(eventtypes.Message)
	mu       sync.Mutex
}

func (w *eventHandler) Handle(action string, h func(eventtypes.Message)) {
	w.mu.Lock()
	w.handlers[action] = h
	w.mu.Unlock()
}

// Watch ranges over the passed in event chan and processes the events based on the
// handlers created for a given action.
// To stop watching, close the event chan.
func (w *eventHandler) Watch(c <-chan eventtypes.Message) {
	for e := range c {
		w.mu.Lock()
		h, exists := w.handlers[e.Action]
		w.mu.Unlock()
		if !exists {
			continue
		}
		logrus.Debugf("event handler: received event: %v", e)
		go h(e)
	}
}

// DecodeEvents decodes event from input stream
func DecodeEvents(input io.Reader, ep eventProcessor) error {
	dec := json.NewDecoder(input)
	for {
		var event eventtypes.Message
		err := dec.Decode(&event)
		if err != nil && err == io.EOF {
			break
		}

		if procErr := ep(event, err); procErr != nil {
			return procErr
		}
	}
	return nil
}
