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
	"errors"
	"fmt"
	"io"

	streamingapi "github.com/containerd/containerd/api/services/streaming/v1"
	"github.com/containerd/errdefs"
	"github.com/containerd/errdefs/pkg/errgrpc"
	"github.com/containerd/ttrpc"
	"github.com/containerd/typeurl/v2"
	"google.golang.org/grpc"

	"github.com/containerd/containerd/v2/core/streaming"
)

// NewStreamCreator returns a new stream creator which can communicate over a GRPC
// or TTRPC connection using the containerd streaming API.
func NewStreamCreator(client any) streaming.StreamCreator {
	switch c := client.(type) {
	case streamingapi.StreamingClient:
		return &streamCreator{
			client: convertClient{c},
		}
	case grpc.ClientConnInterface:
		return &streamCreator{
			client: convertClient{streamingapi.NewStreamingClient(c)},
		}
	case streamingapi.TTRPCStreamingClient:
		return &streamCreator{
			client: c,
		}
	case *ttrpc.Client:
		return &streamCreator{
			client: streamingapi.NewTTRPCStreamingClient(c),
		}
	case streaming.StreamCreator:
		return c
	default:
		panic(fmt.Errorf("unsupported stream client %T: %w", client, errdefs.ErrNotImplemented))
	}
}

type convertClient struct {
	streamingapi.StreamingClient
}

func (c convertClient) Stream(ctx context.Context) (streamingapi.TTRPCStreaming_StreamClient, error) {
	return c.StreamingClient.Stream(ctx)
}

type streamCreator struct {
	client streamingapi.TTRPCStreamingClient
}

func (sc *streamCreator) Create(ctx context.Context, id string) (streaming.Stream, error) {
	stream, err := sc.client.Stream(ctx)
	if err != nil {
		return nil, err
	}

	a, err := typeurl.MarshalAny(&streamingapi.StreamInit{
		ID: id,
	})
	if err != nil {
		return nil, err
	}
	err = stream.Send(typeurl.MarshalProto(a))
	if err != nil {
		if !errors.Is(err, io.EOF) {
			err = errgrpc.ToNative(err)
		}
		return nil, err
	}

	// Receive an ack that stream is init and ready
	if _, err = stream.Recv(); err != nil {
		if !errors.Is(err, io.EOF) {
			err = errgrpc.ToNative(err)
		}
		return nil, err
	}

	return &clientStream{
		s: stream,
	}, nil
}

type clientStream struct {
	s streamingapi.TTRPCStreaming_StreamClient
}

func (cs *clientStream) Send(a typeurl.Any) (err error) {
	err = cs.s.Send(typeurl.MarshalProto(a))
	if !errors.Is(err, io.EOF) {
		err = errgrpc.ToNative(err)
	}
	return
}

func (cs *clientStream) Recv() (a typeurl.Any, err error) {
	a, err = cs.s.Recv()
	if !errors.Is(err, io.EOF) {
		err = errgrpc.ToNative(err)
	}
	return
}

func (cs *clientStream) Close() error {
	return cs.s.CloseSend()
}
