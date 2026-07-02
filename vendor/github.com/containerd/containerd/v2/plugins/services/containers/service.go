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

package containers

import (
	"context"
	"io"

	api "github.com/containerd/containerd/api/services/containers/v1"
	ptypes "github.com/containerd/containerd/v2/pkg/protobuf/types"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/containerd/v2/plugins/services"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
	"google.golang.org/grpc"
)

func init() {
	registry.Register(&plugin.Registration{
		Type: plugins.GRPCPlugin,
		ID:   "containers",
		Requires: []plugin.Type{
			plugins.ServicePlugin,
		},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			i, err := ic.GetByID(plugins.ServicePlugin, services.ContainersService)
			if err != nil {
				return nil, err
			}
			return &service{local: i.(api.ContainersClient)}, nil
		},
	})
}

type service struct {
	local api.ContainersClient
	api.UnimplementedContainersServer
}

var _ api.ContainersServer = &service{}

func (s *service) Register(server *grpc.Server) error {
	api.RegisterContainersServer(server, s)
	return nil
}

func (s *service) Get(ctx context.Context, req *api.GetContainerRequest) (*api.GetContainerResponse, error) {
	return s.local.Get(ctx, req)
}

func (s *service) List(ctx context.Context, req *api.ListContainersRequest) (*api.ListContainersResponse, error) {
	return s.local.List(ctx, req)
}

func (s *service) ListStream(req *api.ListContainersRequest, stream api.Containers_ListStreamServer) error {
	containers, err := s.local.ListStream(stream.Context(), req)
	if err != nil {
		return err
	}
	for {
		select {
		case <-stream.Context().Done():
			return nil
		default:
			c, err := containers.Recv()
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
			if err := stream.Send(c); err != nil {
				return err
			}
		}
	}
}

func (s *service) Create(ctx context.Context, req *api.CreateContainerRequest) (*api.CreateContainerResponse, error) {
	return s.local.Create(ctx, req)
}

func (s *service) Update(ctx context.Context, req *api.UpdateContainerRequest) (*api.UpdateContainerResponse, error) {
	return s.local.Update(ctx, req)
}

func (s *service) Delete(ctx context.Context, req *api.DeleteContainerRequest) (*ptypes.Empty, error) {
	return s.local.Delete(ctx, req)
}
