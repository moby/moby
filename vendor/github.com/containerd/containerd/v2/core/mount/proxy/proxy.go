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
	"fmt"
	"io"

	"github.com/containerd/containerd/api/services/mounts/v1"
	"github.com/containerd/errdefs"
	"github.com/containerd/errdefs/pkg/errgrpc"
	"github.com/containerd/ttrpc"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/containerd/containerd/v2/core/mount"
	ptypes "github.com/containerd/containerd/v2/pkg/protobuf/types"
)

type proxyMounts struct {
	// client is the rpc mounts client
	// NOTE: ttrpc is used because it is the smaller interface shared with grpc
	client mounts.TTRPCMountsClient
}

// NewMountManager returns a new mount manager which communicates over a GRPC
// connection using the containerd mounts GRPC or ttrpc API.
func NewMountManager(client any) mount.Manager {
	switch c := client.(type) {
	case mounts.MountsClient:
		return &proxyMounts{
			client: convertClient{c},
		}
	case grpc.ClientConnInterface:
		return &proxyMounts{
			client: convertClient{mounts.NewMountsClient(c)},
		}
	case mounts.TTRPCMountsClient:
		return &proxyMounts{
			client: c,
		}
	case *ttrpc.Client:
		return &proxyMounts{
			client: mounts.NewTTRPCMountsClient(c),
		}
	default:
		panic(fmt.Errorf("unsupported content client %T: %w", client, errdefs.ErrNotImplemented))
	}
}

func (pm *proxyMounts) Activate(ctx context.Context, name string, all []mount.Mount, opts ...mount.ActivateOpt) (mount.ActivationInfo, error) {
	var options mount.ActivateOptions
	for _, o := range opts {
		o(&options)
	}

	req := &mounts.ActivateRequest{
		Name:      name,
		Mounts:    mount.ToProto(all),
		Labels:    options.Labels,
		Temporary: options.Temporary,
	}

	a, err := pm.client.Activate(ctx, req)
	if err != nil {
		return mount.ActivationInfo{}, errgrpc.ToNative(err)
	}
	return ActivationInfoFromProto(a.Info), nil
}

func (pm *proxyMounts) Deactivate(ctx context.Context, name string) error {
	_, err := pm.client.Deactivate(ctx, &mounts.DeactivateRequest{
		Name: name,
	})
	return errgrpc.ToNative(err)
}

func (pm *proxyMounts) Info(ctx context.Context, name string) (mount.ActivationInfo, error) {
	a, err := pm.client.Info(ctx, &mounts.InfoRequest{
		Name: name,
	})
	if err != nil {
		return mount.ActivationInfo{}, errgrpc.ToNative(err)
	}
	return ActivationInfoFromProto(a.Info), nil
}

func (pm *proxyMounts) Update(ctx context.Context, info mount.ActivationInfo, fieldpaths ...string) (mount.ActivationInfo, error) {
	var updateMask *ptypes.FieldMask
	if len(fieldpaths) > 0 {
		updateMask = &ptypes.FieldMask{
			Paths: fieldpaths,
		}
	}

	a, err := pm.client.Update(ctx, &mounts.UpdateRequest{
		Info:       ActivationInfoToProto(info),
		UpdateMask: updateMask,
	})
	if err != nil {
		return mount.ActivationInfo{}, errgrpc.ToNative(err)
	}
	return ActivationInfoFromProto(a.Info), nil
}

func (pm *proxyMounts) List(ctx context.Context, filters ...string) ([]mount.ActivationInfo, error) {
	l, err := pm.client.List(ctx, &mounts.ListRequest{
		Filters: filters,
	})
	if err != nil {
		return nil, errgrpc.ToNative(err)
	}

	var infos []mount.ActivationInfo
	for {
		a, err := l.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, errgrpc.ToNative(err)
		}
		infos = append(infos, ActivationInfoFromProto(a.Info))
	}
	return infos, nil
}

type convertClient struct {
	mounts.MountsClient
}

func (cc convertClient) Activate(ctx context.Context, req *mounts.ActivateRequest) (*mounts.ActivateResponse, error) {
	return cc.MountsClient.Activate(ctx, req)
}

func (cc convertClient) Deactivate(ctx context.Context, req *mounts.DeactivateRequest) (*emptypb.Empty, error) {
	return cc.MountsClient.Deactivate(ctx, req)
}

func (cc convertClient) Info(ctx context.Context, req *mounts.InfoRequest) (*mounts.InfoResponse, error) {
	return cc.MountsClient.Info(ctx, req)
}

func (cc convertClient) Update(ctx context.Context, req *mounts.UpdateRequest) (*mounts.UpdateResponse, error) {
	return cc.MountsClient.Update(ctx, req)
}

func (cc convertClient) List(ctx context.Context, req *mounts.ListRequest) (mounts.TTRPCMounts_ListClient, error) {
	return cc.MountsClient.List(ctx, req)
}
