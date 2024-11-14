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

package introspectionproxy

import (
	"context"
	"fmt"

	api "github.com/containerd/containerd/api/services/introspection/v1"
	"github.com/containerd/errdefs"
	"github.com/containerd/errdefs/pkg/errgrpc"
	"github.com/containerd/log"
	"github.com/containerd/ttrpc"
	"github.com/containerd/typeurl/v2"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/containerd/containerd/v2/core/introspection"
)

var _ = (introspection.Service)(&introspectionRemote{})

// NewIntrospectionServiceFromClient creates a new introspection service from an API client
func NewIntrospectionProxy(client any) introspection.Service {
	switch c := client.(type) {
	case api.IntrospectionClient:
		return &introspectionRemote{client: convertIntrospection{c}}
	case api.TTRPCIntrospectionService:
		return &introspectionRemote{client: c}
	case grpc.ClientConnInterface:
		return &introspectionRemote{client: convertIntrospection{api.NewIntrospectionClient(c)}}
	case *ttrpc.Client:
		return &introspectionRemote{client: api.NewTTRPCIntrospectionClient(c)}
	default:
		panic(fmt.Errorf("unsupported introspection client %T: %w", client, errdefs.ErrNotImplemented))
	}
}

type introspectionRemote struct {
	client api.TTRPCIntrospectionService
}

func (i *introspectionRemote) Plugins(ctx context.Context, filters ...string) (*api.PluginsResponse, error) {
	log.G(ctx).WithField("filters", filters).Debug("remote introspection plugin filters")
	resp, err := i.client.Plugins(ctx, &api.PluginsRequest{
		Filters: filters,
	})

	if err != nil {
		return nil, errgrpc.ToNative(err)
	}

	return resp, nil
}

func (i *introspectionRemote) Server(ctx context.Context) (*api.ServerResponse, error) {
	resp, err := i.client.Server(ctx, &emptypb.Empty{})

	if err != nil {
		return nil, errgrpc.ToNative(err)
	}

	return resp, nil
}

func (i *introspectionRemote) PluginInfo(ctx context.Context, pluginType, id string, options any) (resp *api.PluginInfoResponse, err error) {
	var optionsPB *anypb.Any
	if options != nil {
		optionsPB, err = typeurl.MarshalAnyToProto(options)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal runtime requst: %w", err)
		}
	}
	resp, err = i.client.PluginInfo(ctx, &api.PluginInfoRequest{
		Type:    pluginType,
		ID:      id,
		Options: optionsPB,
	})

	return resp, errgrpc.ToNative(err)
}

type convertIntrospection struct {
	client api.IntrospectionClient
}

func (c convertIntrospection) Plugins(ctx context.Context, req *api.PluginsRequest) (*api.PluginsResponse, error) {
	return c.client.Plugins(ctx, req)
}
func (c convertIntrospection) Server(ctx context.Context, in *emptypb.Empty) (*api.ServerResponse, error) {
	return c.client.Server(ctx, in)
}
func (c convertIntrospection) PluginInfo(ctx context.Context, req *api.PluginInfoRequest) (*api.PluginInfoResponse, error) {
	return c.client.PluginInfo(ctx, req)
}
