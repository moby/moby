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

package introspection

import (
	context "context"
	"errors"

	"google.golang.org/grpc"

	api "github.com/containerd/containerd/api/services/introspection/v1"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/plugin"
	ptypes "github.com/containerd/containerd/protobuf/types"
	"github.com/containerd/containerd/services"
)

func init() {
	plugin.Register(&plugin.Registration{
		Type:     plugin.GRPCPlugin,
		ID:       "introspection",
		Requires: []plugin.Type{plugin.ServicePlugin},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			plugins, err := ic.GetByType(plugin.ServicePlugin)
			if err != nil {
				return nil, err
			}
			p, ok := plugins[services.IntrospectionService]
			if !ok {
				return nil, errors.New("introspection service not found")
			}

			i, err := p.Instance()
			if err != nil {
				return nil, err
			}

			localClient, ok := i.(*Local)
			if !ok {
				return nil, errors.New("could not create a local client for introspection service")
			}
			localClient.UpdateLocal(ic.Root)

			return &server{
				local: localClient,
			}, nil
		},
	})
}

type server struct {
	local api.IntrospectionClient
	api.UnimplementedIntrospectionServer
}

var _ = (api.IntrospectionServer)(&server{})

func (s *server) Register(server *grpc.Server) error {
	api.RegisterIntrospectionServer(server, s)
	return nil
}

func (s *server) Plugins(ctx context.Context, req *api.PluginsRequest) (*api.PluginsResponse, error) {
	return s.local.Plugins(ctx, req)
}

func (s *server) Server(ctx context.Context, empty *ptypes.Empty) (*api.ServerResponse, error) {
	return s.local.Server(ctx, empty)
}

func (s *server) PluginInfo(ctx context.Context, in *api.PluginInfoRequest) (*api.PluginInfoResponse, error) {
	return nil, errdefs.ToGRPC(errdefs.ErrNotImplemented)
}
