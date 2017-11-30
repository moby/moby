package events

import (
	"context"
	"time"

	"github.com/containerd/typeurl"
	"github.com/gogo/protobuf/types"
)

// Envelope provides the packaging for an event.
type Envelope struct {
	Timestamp time.Time
	Namespace string
	Topic     string
	Event     *types.Any
}

// Field returns the value for the given fieldpath as a string, if defined.
// If the value is not defined, the second value will be false.
func (e *Envelope) Field(fieldpath []string) (string, bool) {
	if len(fieldpath) == 0 {
		return "", false
	}

	switch fieldpath[0] {
	// unhandled: timestamp
	case "namespace":
		return string(e.Namespace), len(e.Namespace) > 0
	case "topic":
		return string(e.Topic), len(e.Topic) > 0
	case "event":
		decoded, err := typeurl.UnmarshalAny(e.Event)
		if err != nil {
			return "", false
		}

		adaptor, ok := decoded.(interface {
			Field([]string) (string, bool)
		})
		if !ok {
			return "", false
		}
		return adaptor.Field(fieldpath[1:])
	}
	return "", false
}

// Event is a generic interface for any type of event
type Event interface{}

// Publisher posts the event.
type Publisher interface {
	Publish(ctx context.Context, topic string, event Event) error
}

// Forwarder forwards an event to the underlying event bus
type Forwarder interface {
	Forward(ctx context.Context, envelope *Envelope) error
}

// Subscriber allows callers to subscribe to events
type Subscriber interface {
	Subscribe(ctx context.Context, filters ...string) (ch <-chan *Envelope, errs <-chan error)
}
