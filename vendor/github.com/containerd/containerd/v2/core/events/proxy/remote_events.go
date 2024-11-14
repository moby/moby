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

package proxy

import (
	"context"
	"fmt"

	api "github.com/containerd/containerd/api/services/events/v1"
	"github.com/containerd/containerd/api/types"
	"github.com/containerd/errdefs"
	"github.com/containerd/errdefs/pkg/errgrpc"
	"github.com/containerd/ttrpc"
	"github.com/containerd/typeurl/v2"
	"google.golang.org/grpc"

	"github.com/containerd/containerd/v2/core/events"
	"github.com/containerd/containerd/v2/pkg/protobuf"
)

type EventService interface {
	events.Publisher
	events.Forwarder
	events.Subscriber
}

func NewRemoteEvents(client any) EventService {
	switch c := client.(type) {
	case api.EventsClient:
		return &grpcEventsProxy{
			client: c,
		}
	case api.TTRPCEventsClient:
		return &ttrpcEventsProxy{
			client: c,
		}
	case grpc.ClientConnInterface:
		return &grpcEventsProxy{
			client: api.NewEventsClient(c),
		}
	case *ttrpc.Client:
		return &ttrpcEventsProxy{
			client: api.NewTTRPCEventsClient(c),
		}
	default:
		panic(fmt.Errorf("unsupported events client %T: %w", client, errdefs.ErrNotImplemented))
	}
}

type grpcEventsProxy struct {
	client api.EventsClient
}

func (p *grpcEventsProxy) Publish(ctx context.Context, topic string, event events.Event) error {
	evt, err := typeurl.MarshalAny(event)
	if err != nil {
		return err
	}
	req := &api.PublishRequest{
		Topic: topic,
		Event: typeurl.MarshalProto(evt),
	}
	if _, err := p.client.Publish(ctx, req); err != nil {
		return errgrpc.ToNative(err)
	}
	return nil
}

func (p *grpcEventsProxy) Forward(ctx context.Context, envelope *events.Envelope) error {
	req := &api.ForwardRequest{
		Envelope: &types.Envelope{
			Timestamp: protobuf.ToTimestamp(envelope.Timestamp),
			Namespace: envelope.Namespace,
			Topic:     envelope.Topic,
			Event:     typeurl.MarshalProto(envelope.Event),
		},
	}
	if _, err := p.client.Forward(ctx, req); err != nil {
		return errgrpc.ToNative(err)
	}
	return nil
}

func (p *grpcEventsProxy) Subscribe(ctx context.Context, filters ...string) (ch <-chan *events.Envelope, errs <-chan error) {
	var (
		evq  = make(chan *events.Envelope)
		errq = make(chan error, 1)
	)

	errs = errq
	ch = evq

	session, err := p.client.Subscribe(ctx, &api.SubscribeRequest{
		Filters: filters,
	})
	if err != nil {
		errq <- err
		close(errq)
		return
	}

	go func() {
		defer close(errq)

		for {
			ev, err := session.Recv()
			if err != nil {
				errq <- err
				return
			}

			select {
			case evq <- &events.Envelope{
				Timestamp: protobuf.FromTimestamp(ev.Timestamp),
				Namespace: ev.Namespace,
				Topic:     ev.Topic,
				Event:     ev.Event,
			}:
			case <-ctx.Done():
				if cerr := ctx.Err(); cerr != context.Canceled {
					errq <- cerr
				}
				return
			}
		}
	}()

	return ch, errs
}

type ttrpcEventsProxy struct {
	client api.TTRPCEventsClient
}

func (p *ttrpcEventsProxy) Publish(ctx context.Context, topic string, event events.Event) error {
	evt, err := typeurl.MarshalAny(event)
	if err != nil {
		return err
	}
	req := &api.PublishRequest{
		Topic: topic,
		Event: typeurl.MarshalProto(evt),
	}
	if _, err := p.client.Publish(ctx, req); err != nil {
		return errgrpc.ToNative(err)
	}
	return nil
}

func (p *ttrpcEventsProxy) Forward(ctx context.Context, envelope *events.Envelope) error {
	req := &api.ForwardRequest{
		Envelope: &types.Envelope{
			Timestamp: protobuf.ToTimestamp(envelope.Timestamp),
			Namespace: envelope.Namespace,
			Topic:     envelope.Topic,
			Event:     typeurl.MarshalProto(envelope.Event),
		},
	}
	if _, err := p.client.Forward(ctx, req); err != nil {
		return errgrpc.ToNative(err)
	}
	return nil
}

func (p *ttrpcEventsProxy) Subscribe(ctx context.Context, filters ...string) (ch <-chan *events.Envelope, errs <-chan error) {
	var (
		evq  = make(chan *events.Envelope)
		errq = make(chan error, 1)
	)

	errs = errq
	ch = evq

	session, err := p.client.Subscribe(ctx, &api.SubscribeRequest{
		Filters: filters,
	})
	if err != nil {
		errq <- err
		close(errq)
		return
	}

	go func() {
		defer close(errq)

		for {
			ev, err := session.Recv()
			if err != nil {
				errq <- err
				return
			}

			select {
			case evq <- &events.Envelope{
				Timestamp: protobuf.FromTimestamp(ev.Timestamp),
				Namespace: ev.Namespace,
				Topic:     ev.Topic,
				Event:     ev.Event,
			}:
			case <-ctx.Done():
				if cerr := ctx.Err(); cerr != context.Canceled {
					errq <- cerr
				}
				return
			}
		}
	}()

	return ch, errs
}
