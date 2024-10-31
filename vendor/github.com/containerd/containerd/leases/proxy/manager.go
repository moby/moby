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

	leasesapi "github.com/containerd/containerd/api/services/leases/v1"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/protobuf"
)

type proxyManager struct {
	client leasesapi.LeasesClient
}

// NewLeaseManager returns a lease manager which communicates
// through a grpc lease service.
func NewLeaseManager(client leasesapi.LeasesClient) leases.Manager {
	return &proxyManager{
		client: client,
	}
}

func (pm *proxyManager) Create(ctx context.Context, opts ...leases.Opt) (leases.Lease, error) {
	l := leases.Lease{}
	for _, opt := range opts {
		if err := opt(&l); err != nil {
			return leases.Lease{}, err
		}
	}
	resp, err := pm.client.Create(ctx, &leasesapi.CreateRequest{
		ID:     l.ID,
		Labels: l.Labels,
	})
	if err != nil {
		return leases.Lease{}, errdefs.FromGRPC(err)
	}

	return leases.Lease{
		ID:        resp.Lease.ID,
		CreatedAt: protobuf.FromTimestamp(resp.Lease.CreatedAt),
		Labels:    resp.Lease.Labels,
	}, nil
}

func (pm *proxyManager) Delete(ctx context.Context, l leases.Lease, opts ...leases.DeleteOpt) error {
	var do leases.DeleteOptions
	for _, opt := range opts {
		if err := opt(ctx, &do); err != nil {
			return err
		}
	}

	_, err := pm.client.Delete(ctx, &leasesapi.DeleteRequest{
		ID:   l.ID,
		Sync: do.Synchronous,
	})
	return errdefs.FromGRPC(err)
}

func (pm *proxyManager) List(ctx context.Context, filters ...string) ([]leases.Lease, error) {
	resp, err := pm.client.List(ctx, &leasesapi.ListRequest{
		Filters: filters,
	})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}
	l := make([]leases.Lease, len(resp.Leases))
	for i := range resp.Leases {
		l[i] = leases.Lease{
			ID:        resp.Leases[i].ID,
			CreatedAt: protobuf.FromTimestamp(resp.Leases[i].CreatedAt),
			Labels:    resp.Leases[i].Labels,
		}
	}

	return l, nil
}

func (pm *proxyManager) AddResource(ctx context.Context, lease leases.Lease, r leases.Resource) error {
	_, err := pm.client.AddResource(ctx, &leasesapi.AddResourceRequest{
		ID: lease.ID,
		Resource: &leasesapi.Resource{
			ID:   r.ID,
			Type: r.Type,
		},
	})
	return errdefs.FromGRPC(err)
}

func (pm *proxyManager) DeleteResource(ctx context.Context, lease leases.Lease, r leases.Resource) error {
	_, err := pm.client.DeleteResource(ctx, &leasesapi.DeleteResourceRequest{
		ID: lease.ID,
		Resource: &leasesapi.Resource{
			ID:   r.ID,
			Type: r.Type,
		},
	})
	return errdefs.FromGRPC(err)
}

func (pm *proxyManager) ListResources(ctx context.Context, lease leases.Lease) ([]leases.Resource, error) {
	resp, err := pm.client.ListResources(ctx, &leasesapi.ListResourcesRequest{
		ID: lease.ID,
	})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}

	rs := make([]leases.Resource, 0, len(resp.Resources))
	for _, i := range resp.Resources {
		rs = append(rs, leases.Resource{
			ID:   i.ID,
			Type: i.Type,
		})
	}
	return rs, nil
}
