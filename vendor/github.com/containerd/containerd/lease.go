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
	"time"

	"github.com/containerd/containerd/leases"
)

// WithLease attaches a lease on the context
func (c *Client) WithLease(ctx context.Context) (context.Context, func(context.Context) error, error) {
	_, ok := leases.FromContext(ctx)
	if ok {
		return ctx, func(context.Context) error {
			return nil
		}, nil
	}

	ls := c.LeasesService()

	l, err := ls.Create(ctx, leases.WithRandomID(), leases.WithExpiration(24*3600*time.Second))
	if err != nil {
		return nil, nil, err
	}

	ctx = leases.WithLease(ctx, l.ID)
	return ctx, func(ctx context.Context) error {
		return ls.Delete(ctx, l)
	}, nil
}
