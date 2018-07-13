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

	leasesapi "github.com/containerd/containerd/api/services/leases/v1"
	"github.com/containerd/containerd/leases"
)

// Lease is used to hold a reference to active resources which have not been
// referenced by a root resource. This is useful for preventing garbage
// collection of resources while they are actively being updated.
type Lease struct {
	id        string
	createdAt time.Time

	client *Client
}

// CreateLease creates a new lease
func (c *Client) CreateLease(ctx context.Context) (Lease, error) {
	lapi := c.LeasesService()
	resp, err := lapi.Create(ctx, &leasesapi.CreateRequest{})
	if err != nil {
		return Lease{}, err
	}

	return Lease{
		id:     resp.Lease.ID,
		client: c,
	}, nil
}

// ListLeases lists active leases
func (c *Client) ListLeases(ctx context.Context) ([]Lease, error) {
	lapi := c.LeasesService()
	resp, err := lapi.List(ctx, &leasesapi.ListRequest{})
	if err != nil {
		return nil, err
	}
	leases := make([]Lease, len(resp.Leases))
	for i := range resp.Leases {
		leases[i] = Lease{
			id:        resp.Leases[i].ID,
			createdAt: resp.Leases[i].CreatedAt,
			client:    c,
		}
	}

	return leases, nil
}

// WithLease attaches a lease on the context
func (c *Client) WithLease(ctx context.Context) (context.Context, func(context.Context) error, error) {
	_, ok := leases.Lease(ctx)
	if ok {
		return ctx, func(context.Context) error {
			return nil
		}, nil
	}

	l, err := c.CreateLease(ctx)
	if err != nil {
		return nil, nil, err
	}

	ctx = leases.WithLease(ctx, l.ID())
	return ctx, func(ctx context.Context) error {
		return l.Delete(ctx)
	}, nil
}

// ID returns the lease ID
func (l Lease) ID() string {
	return l.id
}

// CreatedAt returns the time at which the lease was created
func (l Lease) CreatedAt() time.Time {
	return l.createdAt
}

// Delete deletes the lease, removing the reference to all resources created
// during the lease.
func (l Lease) Delete(ctx context.Context) error {
	lapi := l.client.LeasesService()
	_, err := lapi.Delete(ctx, &leasesapi.DeleteRequest{
		ID: l.id,
	})
	return err
}
