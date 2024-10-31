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

package containerd

import (
	"context"
	"errors"
	"io"

	streamingapi "github.com/containerd/containerd/api/services/streaming/v1"
	transferapi "github.com/containerd/containerd/api/services/transfer/v1"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/pkg/streaming"
	"github.com/containerd/containerd/pkg/transfer"
	"github.com/containerd/containerd/pkg/transfer/proxy"
	"github.com/containerd/containerd/protobuf"
	"github.com/containerd/typeurl/v2"
)

func (c *Client) Transfer(ctx context.Context, src interface{}, dest interface{}, opts ...transfer.Opt) error {
	ctx, done, err := c.WithLease(ctx)
	if err != nil {
		return err
	}
	defer done(ctx)

	return proxy.NewTransferrer(transferapi.NewTransferClient(c.conn), c.streamCreator()).Transfer(ctx, src, dest, opts...)
}

func (c *Client) streamCreator() streaming.StreamCreator {
	return &streamCreator{
		client: streamingapi.NewStreamingClient(c.conn),
	}
}

type streamCreator struct {
	client streamingapi.StreamingClient
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
	err = stream.Send(protobuf.FromAny(a))
	if err != nil {
		if !errors.Is(err, io.EOF) {
			err = errdefs.FromGRPC(err)
		}
		return nil, err
	}

	// Receive an ack that stream is init and ready
	if _, err = stream.Recv(); err != nil {
		if !errors.Is(err, io.EOF) {
			err = errdefs.FromGRPC(err)
		}
		return nil, err
	}

	return &clientStream{
		s: stream,
	}, nil
}

type clientStream struct {
	s streamingapi.Streaming_StreamClient
}

func (cs *clientStream) Send(a typeurl.Any) (err error) {
	err = cs.s.Send(protobuf.FromAny(a))
	if !errors.Is(err, io.EOF) {
		err = errdefs.FromGRPC(err)
	}
	return
}

func (cs *clientStream) Recv() (a typeurl.Any, err error) {
	a, err = cs.s.Recv()
	if !errors.Is(err, io.EOF) {
		err = errdefs.FromGRPC(err)
	}
	return
}

func (cs *clientStream) Close() error {
	return cs.s.CloseSend()
}
