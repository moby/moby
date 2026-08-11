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

package streaming

import (
	"context"
	"errors"
	"fmt"
	"io"

	transferapi "github.com/containerd/containerd/api/types/transfer"
	"github.com/containerd/containerd/v2/core/streaming"
	"github.com/containerd/typeurl/v2"
)

type readByteStream struct {
	ctx       context.Context
	stream    streaming.Stream
	window    int32
	updated   chan struct{}
	errCh     chan error
	remaining []byte
}

func ReadByteStream(ctx context.Context, stream streaming.Stream) io.ReadCloser {
	rbs := &readByteStream{
		ctx:     ctx,
		stream:  stream,
		window:  0,
		errCh:   make(chan error),
		updated: make(chan struct{}, 1),
	}
	go func() {
		for {
			if rbs.window >= windowSize {
				select {
				case <-ctx.Done():
					return
				case <-rbs.updated:
					continue
				}
			}
			update := &transferapi.WindowUpdate{
				Update: windowSize,
			}
			anyType, err := typeurl.MarshalAny(update)
			if err != nil {
				rbs.errCh <- err
				return
			}
			if err := stream.Send(anyType); err == nil {
				rbs.window += windowSize
			} else if !errors.Is(err, io.EOF) {
				rbs.errCh <- err
			}
		}

	}()
	return rbs
}

func (r *readByteStream) Read(p []byte) (n int, err error) {
	plen := len(p)
	if len(r.remaining) > 0 {
		copied := copy(p, r.remaining)
		if len(r.remaining) > plen {
			r.remaining = r.remaining[plen:]
		} else {
			r.remaining = nil
		}
		return copied, nil
	}
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	case err := <-r.errCh:
		return 0, err
	default:
	}
	anyType, err := r.stream.Recv()
	if err != nil {
		return 0, err
	}
	i, err := typeurl.UnmarshalAny(anyType)
	if err != nil {
		return 0, err
	}
	switch v := i.(type) {
	case *transferapi.Data:
		n := copy(p, v.Data)
		if len(v.Data) > plen {
			r.remaining = v.Data[plen:]
		}
		r.window = r.window - int32(n)
		if r.window < windowSize {
			r.updated <- struct{}{}
		}
		return n, nil
	default:
		return 0, fmt.Errorf("stream received error type %v", v)
	}

}

func (r *readByteStream) Close() error {
	return r.stream.Close()
}
