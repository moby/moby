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

package leases

import (
	"context"

	api "github.com/containerd/containerd/api/services/leases/v1"
	"github.com/containerd/errdefs/pkg/errgrpc"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
	"google.golang.org/grpc"

	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/containerd/v2/pkg/protobuf"
	ptypes "github.com/containerd/containerd/v2/pkg/protobuf/types"
	"github.com/containerd/containerd/v2/plugins"
)

var empty = &ptypes.Empty{}

func init() {
	registry.Register(&plugin.Registration{
		Type: plugins.GRPCPlugin,
		ID:   "leases",
		Requires: []plugin.Type{
			plugins.LeasePlugin,
		},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			i, err := ic.GetByID(plugins.LeasePlugin, "manager")
			if err != nil {
				return nil, err
			}
			return &service{lm: i.(leases.Manager)}, nil
		},
	})
}

type service struct {
	lm leases.Manager
	api.UnimplementedLeasesServer
}

func (s *service) Register(server *grpc.Server) error {
	api.RegisterLeasesServer(server, s)
	return nil
}

func (s *service) Create(ctx context.Context, r *api.CreateRequest) (*api.CreateResponse, error) {
	opts := []leases.Opt{
		leases.WithLabels(r.Labels),
	}
	if r.ID == "" {
		opts = append(opts, leases.WithRandomID())
	} else {
		opts = append(opts, leases.WithID(r.ID))
	}

	l, err := s.lm.Create(ctx, opts...)
	if err != nil {
		return nil, errgrpc.ToGRPC(err)
	}

	return &api.CreateResponse{
		Lease: leaseToGRPC(l),
	}, nil
}

func (s *service) Delete(ctx context.Context, r *api.DeleteRequest) (*ptypes.Empty, error) {
	var opts []leases.DeleteOpt
	if r.Sync {
		opts = append(opts, leases.SynchronousDelete)
	}
	if err := s.lm.Delete(ctx, leases.Lease{
		ID: r.ID,
	}, opts...); err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	return empty, nil
}

func (s *service) List(ctx context.Context, r *api.ListRequest) (*api.ListResponse, error) {
	l, err := s.lm.List(ctx, r.Filters...)
	if err != nil {
		return nil, errgrpc.ToGRPC(err)
	}

	apileases := make([]*api.Lease, len(l))
	for i := range l {
		apileases[i] = leaseToGRPC(l[i])
	}

	return &api.ListResponse{
		Leases: apileases,
	}, nil
}

func (s *service) AddResource(ctx context.Context, r *api.AddResourceRequest) (*ptypes.Empty, error) {
	lease := leases.Lease{
		ID: r.ID,
	}

	if err := s.lm.AddResource(ctx, lease, leases.Resource{
		ID:   r.Resource.ID,
		Type: r.Resource.Type,
	}); err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	return empty, nil
}

func (s *service) DeleteResource(ctx context.Context, r *api.DeleteResourceRequest) (*ptypes.Empty, error) {
	lease := leases.Lease{
		ID: r.ID,
	}

	if err := s.lm.DeleteResource(ctx, lease, leases.Resource{
		ID:   r.Resource.ID,
		Type: r.Resource.Type,
	}); err != nil {
		return nil, errgrpc.ToGRPC(err)
	}
	return empty, nil
}

func (s *service) ListResources(ctx context.Context, r *api.ListResourcesRequest) (*api.ListResourcesResponse, error) {
	lease := leases.Lease{
		ID: r.ID,
	}

	rs, err := s.lm.ListResources(ctx, lease)
	if err != nil {
		return nil, errgrpc.ToGRPC(err)
	}

	apiResources := make([]*api.Resource, 0, len(rs))
	for _, i := range rs {
		apiResources = append(apiResources, &api.Resource{
			ID:   i.ID,
			Type: i.Type,
		})
	}
	return &api.ListResourcesResponse{
		Resources: apiResources,
	}, nil
}

func leaseToGRPC(l leases.Lease) *api.Lease {
	return &api.Lease{
		ID:        l.ID,
		Labels:    l.Labels,
		CreatedAt: protobuf.ToTimestamp(l.CreatedAt),
	}
}
