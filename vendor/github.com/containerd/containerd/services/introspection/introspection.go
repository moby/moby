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

	"github.com/containerd/log"

	api "github.com/containerd/containerd/api/services/introspection/v1"
	"github.com/containerd/containerd/errdefs"
	ptypes "github.com/containerd/containerd/protobuf/types"
)

// Service defines the introspection service interface
type Service interface {
	Plugins(context.Context, []string) (*api.PluginsResponse, error)
	Server(context.Context, *ptypes.Empty) (*api.ServerResponse, error)
}

type introspectionRemote struct {
	client api.IntrospectionClient
}

var _ = (Service)(&introspectionRemote{})

// NewIntrospectionServiceFromClient creates a new introspection service from an API client
func NewIntrospectionServiceFromClient(c api.IntrospectionClient) Service {
	return &introspectionRemote{client: c}
}

func (i *introspectionRemote) Plugins(ctx context.Context, filters []string) (*api.PluginsResponse, error) {
	log.G(ctx).WithField("filters", filters).Debug("remote introspection plugin filters")
	resp, err := i.client.Plugins(ctx, &api.PluginsRequest{
		Filters: filters,
	})

	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}

	return resp, nil
}

func (i *introspectionRemote) Server(ctx context.Context, in *ptypes.Empty) (*api.ServerResponse, error) {
	resp, err := i.client.Server(ctx, in)

	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}

	return resp, nil
}
