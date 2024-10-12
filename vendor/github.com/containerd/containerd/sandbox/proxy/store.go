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

	api "github.com/containerd/containerd/api/services/sandbox/v1"
	"github.com/containerd/containerd/errdefs"
	sb "github.com/containerd/containerd/sandbox"
)

// remoteSandboxStore is a low-level containerd client to manage sandbox environments metadata
type remoteSandboxStore struct {
	client api.StoreClient
}

var _ sb.Store = (*remoteSandboxStore)(nil)

// NewSandboxStore create a client for a sandbox store
func NewSandboxStore(client api.StoreClient) sb.Store {
	return &remoteSandboxStore{client: client}
}

func (s *remoteSandboxStore) Create(ctx context.Context, sandbox sb.Sandbox) (sb.Sandbox, error) {
	resp, err := s.client.Create(ctx, &api.StoreCreateRequest{
		Sandbox: sb.ToProto(&sandbox),
	})
	if err != nil {
		return sb.Sandbox{}, errdefs.FromGRPC(err)
	}

	return sb.FromProto(resp.Sandbox), nil
}

func (s *remoteSandboxStore) Update(ctx context.Context, sandbox sb.Sandbox, fieldpaths ...string) (sb.Sandbox, error) {
	resp, err := s.client.Update(ctx, &api.StoreUpdateRequest{
		Sandbox: sb.ToProto(&sandbox),
		Fields:  fieldpaths,
	})
	if err != nil {
		return sb.Sandbox{}, errdefs.FromGRPC(err)
	}

	return sb.FromProto(resp.Sandbox), nil
}

func (s *remoteSandboxStore) Get(ctx context.Context, id string) (sb.Sandbox, error) {
	resp, err := s.client.Get(ctx, &api.StoreGetRequest{
		SandboxID: id,
	})
	if err != nil {
		return sb.Sandbox{}, errdefs.FromGRPC(err)
	}

	return sb.FromProto(resp.Sandbox), nil
}

func (s *remoteSandboxStore) List(ctx context.Context, filters ...string) ([]sb.Sandbox, error) {
	resp, err := s.client.List(ctx, &api.StoreListRequest{
		Filters: filters,
	})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}

	out := make([]sb.Sandbox, len(resp.List))
	for i := range resp.List {
		out[i] = sb.FromProto(resp.List[i])
	}

	return out, nil
}

func (s *remoteSandboxStore) Delete(ctx context.Context, id string) error {
	_, err := s.client.Delete(ctx, &api.StoreDeleteRequest{
		SandboxID: id,
	})

	return errdefs.FromGRPC(err)
}
