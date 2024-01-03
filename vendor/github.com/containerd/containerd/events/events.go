/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package events

import (
	"context"
	"time"

	"github.com/containerd/typeurl/v2"
)

// Envelope provides the packaging for an event.
type Envelope struct {
	Timestamp time.Time
	Namespace string
	Topic     string
	Event     typeurl.Any
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
		return e.Namespace, len(e.Namespace) > 0
	case "topic":
		return e.Topic, len(e.Topic) > 0
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
