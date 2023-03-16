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

package exchange

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/filters"
	"github.com/containerd/containerd/identifiers"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/typeurl/v2"
	goevents "github.com/docker/go-events"
)

// Exchange broadcasts events
type Exchange struct {
	broadcaster *goevents.Broadcaster
}

// NewExchange returns a new event Exchange
func NewExchange() *Exchange {
	return &Exchange{
		broadcaster: goevents.NewBroadcaster(),
	}
}

var _ events.Publisher = &Exchange{}
var _ events.Forwarder = &Exchange{}
var _ events.Subscriber = &Exchange{}

// Forward accepts an envelope to be directly distributed on the exchange.
//
// This is useful when an event is forwarded on behalf of another namespace or
// when the event is propagated on behalf of another publisher.
func (e *Exchange) Forward(ctx context.Context, envelope *events.Envelope) (err error) {
	if err := validateEnvelope(envelope); err != nil {
		return err
	}

	defer func() {
		logger := log.G(ctx).WithFields(log.Fields{
			"topic": envelope.Topic,
			"ns":    envelope.Namespace,
			"type":  envelope.Event.GetTypeUrl(),
		})

		if err != nil {
			logger.WithError(err).Error("error forwarding event")
		} else {
			logger.Debug("event forwarded")
		}
	}()

	return e.broadcaster.Write(envelope)
}

// Publish packages and sends an event. The caller will be considered the
// initial publisher of the event. This means the timestamp will be calculated
// at this point and this method may read from the calling context.
func (e *Exchange) Publish(ctx context.Context, topic string, event events.Event) (err error) {
	var (
		namespace string
		envelope  events.Envelope
	)

	namespace, err = namespaces.NamespaceRequired(ctx)
	if err != nil {
		return fmt.Errorf("failed publishing event: %w", err)
	}
	if err := validateTopic(topic); err != nil {
		return fmt.Errorf("envelope topic %q: %w", topic, err)
	}

	encoded, err := typeurl.MarshalAny(event)
	if err != nil {
		return err
	}

	envelope.Timestamp = time.Now().UTC()
	envelope.Namespace = namespace
	envelope.Topic = topic
	envelope.Event = encoded

	defer func() {
		logger := log.G(ctx).WithFields(log.Fields{
			"topic": envelope.Topic,
			"ns":    envelope.Namespace,
			"type":  envelope.Event.GetTypeUrl(),
		})

		if err != nil {
			logger.WithError(err).Error("error publishing event")
		} else {
			logger.Debug("event published")
		}
	}()

	return e.broadcaster.Write(&envelope)
}

// Subscribe to events on the exchange. Events are sent through the returned
// channel ch. If an error is encountered, it will be sent on channel errs and
// errs will be closed. To end the subscription, cancel the provided context.
//
// Zero or more filters may be provided as strings. Only events that match
// *any* of the provided filters will be sent on the channel. The filters use
// the standard containerd filters package syntax.
func (e *Exchange) Subscribe(ctx context.Context, fs ...string) (ch <-chan *events.Envelope, errs <-chan error) {
	var (
		evch                  = make(chan *events.Envelope)
		errq                  = make(chan error, 1)
		channel               = goevents.NewChannel(0)
		queue                 = goevents.NewQueue(channel)
		dst     goevents.Sink = queue
	)

	closeAll := func() {
		channel.Close()
		queue.Close()
		e.broadcaster.Remove(dst)
		close(errq)
	}

	ch = evch
	errs = errq

	if len(fs) > 0 {
		filter, err := filters.ParseAll(fs...)
		if err != nil {
			errq <- fmt.Errorf("failed parsing subscription filters: %w", err)
			closeAll()
			return
		}

		dst = goevents.NewFilter(queue, goevents.MatcherFunc(func(gev goevents.Event) bool {
			return filter.Match(adapt(gev))
		}))
	}

	e.broadcaster.Add(dst)

	go func() {
		defer closeAll()

		var err error
	loop:
		for {
			select {
			case ev := <-channel.C:
				env, ok := ev.(*events.Envelope)
				if !ok {
					// TODO(stevvooe): For the most part, we are well protected
					// from this condition. Both Forward and Publish protect
					// from this.
					err = fmt.Errorf("invalid envelope encountered %#v; please file a bug", ev)
					break
				}

				select {
				case evch <- env:
				case <-ctx.Done():
					break loop
				}
			case <-ctx.Done():
				break loop
			}
		}

		if err == nil {
			if cerr := ctx.Err(); cerr != context.Canceled {
				err = cerr
			}
		}

		errq <- err
	}()

	return
}

func validateTopic(topic string) error {
	if topic == "" {
		return fmt.Errorf("must not be empty: %w", errdefs.ErrInvalidArgument)
	}

	if topic[0] != '/' {
		return fmt.Errorf("must start with '/': %w", errdefs.ErrInvalidArgument)
	}

	if len(topic) == 1 {
		return fmt.Errorf("must have at least one component: %w", errdefs.ErrInvalidArgument)
	}

	components := strings.Split(topic[1:], "/")
	for _, component := range components {
		if err := identifiers.Validate(component); err != nil {
			return fmt.Errorf("failed validation on component %q: %w", component, err)
		}
	}

	return nil
}

func validateEnvelope(envelope *events.Envelope) error {
	if err := identifiers.Validate(envelope.Namespace); err != nil {
		return fmt.Errorf("event envelope has invalid namespace: %w", err)
	}

	if err := validateTopic(envelope.Topic); err != nil {
		return fmt.Errorf("envelope topic %q: %w", envelope.Topic, err)
	}

	if envelope.Timestamp.IsZero() {
		return fmt.Errorf("timestamp must be set on forwarded event: %w", errdefs.ErrInvalidArgument)
	}

	return nil
}

func adapt(ev interface{}) filters.Adaptor {
	if adaptor, ok := ev.(filters.Adaptor); ok {
		return adaptor
	}

	return filters.AdapterFunc(func(fieldpath []string) (string, bool) {
		return "", false
	})
}
