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
	"fmt"

	api "github.com/containerd/containerd/api/services/introspection/v1"
	"github.com/containerd/errdefs/pkg/errgrpc"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
	"github.com/containerd/typeurl/v2"
	"google.golang.org/grpc"

	"github.com/containerd/containerd/v2/core/introspection"
	ptypes "github.com/containerd/containerd/v2/pkg/protobuf/types"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/containerd/v2/plugins/services"
)

func init() {
	registry.Register(&plugin.Registration{
		Type:     plugins.GRPCPlugin,
		ID:       "introspection",
		Requires: []plugin.Type{plugins.ServicePlugin},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			i, err := ic.GetByID(plugins.ServicePlugin, services.IntrospectionService)
			if err != nil {
				return nil, err
			}

			localClient, ok := i.(*Local)
			if !ok {
				return nil, errors.New("could not create a local client for introspection service")
			}
			localClient.UpdateLocal(ic.Properties[plugins.PropertyRootDir])

			return &server{
				local: localClient,
			}, nil
		},
	})
}

type server struct {
	local introspection.Service
	api.UnimplementedIntrospectionServer
}

var _ = (api.IntrospectionServer)(&server{})

func (s *server) Register(server *grpc.Server) error {
	api.RegisterIntrospectionServer(server, s)
	return nil
}

func (s *server) Plugins(ctx context.Context, req *api.PluginsRequest) (resp *api.PluginsResponse, err error) {
	resp, err = s.local.Plugins(ctx, req.Filters...)
	return resp, errgrpc.ToGRPC(err)
}

func (s *server) Server(ctx context.Context, _ *ptypes.Empty) (resp *api.ServerResponse, err error) {
	resp, err = s.local.Server(ctx)
	return resp, errgrpc.ToGRPC(err)
}

func (s *server) PluginInfo(ctx context.Context, req *api.PluginInfoRequest) (resp *api.PluginInfoResponse, err error) {
	var options any
	if req.Options != nil {
		options, err = typeurl.UnmarshalAny(req.Options)
		if err != nil {
			return resp, errgrpc.ToGRPC(fmt.Errorf("failed to unmarshal plugin info Options: %w", err))
		}
	}

	resp, err = s.local.PluginInfo(ctx, req.Type, req.ID, options)
	return resp, errgrpc.ToGRPC(err)
}
