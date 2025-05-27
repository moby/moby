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

package client

import (
	"context"

	"github.com/containerd/containerd/v2/core/streaming"
	streamproxy "github.com/containerd/containerd/v2/core/streaming/proxy"
	"github.com/containerd/containerd/v2/core/transfer"
	"github.com/containerd/containerd/v2/core/transfer/proxy"
)

func (c *Client) Transfer(ctx context.Context, src interface{}, dest interface{}, opts ...transfer.Opt) error {
	ctx, done, err := c.WithLease(ctx)
	if err != nil {
		return err
	}
	defer done(ctx)

	return proxy.NewTransferrer(c.conn, c.streamCreator()).Transfer(ctx, src, dest, opts...)
}

func (c *Client) streamCreator() streaming.StreamCreator {
	return streamproxy.NewStreamCreator(c.conn)
}
