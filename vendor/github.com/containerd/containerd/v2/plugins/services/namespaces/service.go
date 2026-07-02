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

package namespaces

import (
	"context"

	api "github.com/containerd/containerd/api/services/namespaces/v1"
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
		ID:   "namespaces",
		Requires: []plugin.Type{
			plugins.ServicePlugin,
		},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			i, err := ic.GetByID(plugins.ServicePlugin, services.NamespacesService)
			if err != nil {
				return nil, err
			}
			return &service{local: i.(api.NamespacesClient)}, nil
		},
	})
}

type service struct {
	local api.NamespacesClient
	api.UnimplementedNamespacesServer
}

var _ api.NamespacesServer = &service{}

func (s *service) Register(server *grpc.Server) error {
	api.RegisterNamespacesServer(server, s)
	return nil
}

func (s *service) Get(ctx context.Context, req *api.GetNamespaceRequest) (*api.GetNamespaceResponse, error) {
	return s.local.Get(ctx, req)
}

func (s *service) List(ctx context.Context, req *api.ListNamespacesRequest) (*api.ListNamespacesResponse, error) {
	return s.local.List(ctx, req)
}

func (s *service) Create(ctx context.Context, req *api.CreateNamespaceRequest) (*api.CreateNamespaceResponse, error) {
	return s.local.Create(ctx, req)
}

func (s *service) Update(ctx context.Context, req *api.UpdateNamespaceRequest) (*api.UpdateNamespaceResponse, error) {
	return s.local.Update(ctx, req)
}

func (s *service) Delete(ctx context.Context, req *api.DeleteNamespaceRequest) (*ptypes.Empty, error) {
	return s.local.Delete(ctx, req)
}
