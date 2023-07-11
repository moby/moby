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
	"io"

	snapshotsapi "github.com/containerd/containerd/api/services/snapshots/v1"
	"github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/protobuf"
	protobuftypes "github.com/containerd/containerd/protobuf/types"
	"github.com/containerd/containerd/snapshots"
)

// NewSnapshotter returns a new Snapshotter which communicates over a GRPC
// connection using the containerd snapshot GRPC API.
func NewSnapshotter(client snapshotsapi.SnapshotsClient, snapshotterName string) snapshots.Snapshotter {
	return &proxySnapshotter{
		client:          client,
		snapshotterName: snapshotterName,
	}
}

type proxySnapshotter struct {
	client          snapshotsapi.SnapshotsClient
	snapshotterName string
}

func (p *proxySnapshotter) Stat(ctx context.Context, key string) (snapshots.Info, error) {
	resp, err := p.client.Stat(ctx,
		&snapshotsapi.StatSnapshotRequest{
			Snapshotter: p.snapshotterName,
			Key:         key,
		})
	if err != nil {
		return snapshots.Info{}, errdefs.FromGRPC(err)
	}
	return toInfo(resp.Info), nil
}

func (p *proxySnapshotter) Update(ctx context.Context, info snapshots.Info, fieldpaths ...string) (snapshots.Info, error) {
	resp, err := p.client.Update(ctx,
		&snapshotsapi.UpdateSnapshotRequest{
			Snapshotter: p.snapshotterName,
			Info:        fromInfo(info),
			UpdateMask: &protobuftypes.FieldMask{
				Paths: fieldpaths,
			},
		})
	if err != nil {
		return snapshots.Info{}, errdefs.FromGRPC(err)
	}
	return toInfo(resp.Info), nil
}

func (p *proxySnapshotter) Usage(ctx context.Context, key string) (snapshots.Usage, error) {
	resp, err := p.client.Usage(ctx, &snapshotsapi.UsageRequest{
		Snapshotter: p.snapshotterName,
		Key:         key,
	})
	if err != nil {
		return snapshots.Usage{}, errdefs.FromGRPC(err)
	}
	return toUsage(resp), nil
}

func (p *proxySnapshotter) Mounts(ctx context.Context, key string) ([]mount.Mount, error) {
	resp, err := p.client.Mounts(ctx, &snapshotsapi.MountsRequest{
		Snapshotter: p.snapshotterName,
		Key:         key,
	})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}
	return toMounts(resp.Mounts), nil
}

func (p *proxySnapshotter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	var local snapshots.Info
	for _, opt := range opts {
		if err := opt(&local); err != nil {
			return nil, err
		}
	}
	resp, err := p.client.Prepare(ctx, &snapshotsapi.PrepareSnapshotRequest{
		Snapshotter: p.snapshotterName,
		Key:         key,
		Parent:      parent,
		Labels:      local.Labels,
	})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}
	return toMounts(resp.Mounts), nil
}

func (p *proxySnapshotter) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	var local snapshots.Info
	for _, opt := range opts {
		if err := opt(&local); err != nil {
			return nil, err
		}
	}
	resp, err := p.client.View(ctx, &snapshotsapi.ViewSnapshotRequest{
		Snapshotter: p.snapshotterName,
		Key:         key,
		Parent:      parent,
		Labels:      local.Labels,
	})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}
	return toMounts(resp.Mounts), nil
}

func (p *proxySnapshotter) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) error {
	var local snapshots.Info
	for _, opt := range opts {
		if err := opt(&local); err != nil {
			return err
		}
	}
	_, err := p.client.Commit(ctx, &snapshotsapi.CommitSnapshotRequest{
		Snapshotter: p.snapshotterName,
		Name:        name,
		Key:         key,
		Labels:      local.Labels,
	})
	return errdefs.FromGRPC(err)
}

func (p *proxySnapshotter) Remove(ctx context.Context, key string) error {
	_, err := p.client.Remove(ctx, &snapshotsapi.RemoveSnapshotRequest{
		Snapshotter: p.snapshotterName,
		Key:         key,
	})
	return errdefs.FromGRPC(err)
}

func (p *proxySnapshotter) Walk(ctx context.Context, fn snapshots.WalkFunc, fs ...string) error {
	sc, err := p.client.List(ctx, &snapshotsapi.ListSnapshotsRequest{
		Snapshotter: p.snapshotterName,
		Filters:     fs,
	})
	if err != nil {
		return errdefs.FromGRPC(err)
	}
	for {
		resp, err := sc.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return errdefs.FromGRPC(err)
		}
		if resp == nil {
			return nil
		}
		for _, info := range resp.Info {
			if err := fn(ctx, toInfo(info)); err != nil {
				return err
			}
		}
	}
}

func (p *proxySnapshotter) Close() error {
	return nil
}

func (p *proxySnapshotter) Cleanup(ctx context.Context) error {
	_, err := p.client.Cleanup(ctx, &snapshotsapi.CleanupRequest{
		Snapshotter: p.snapshotterName,
	})
	return errdefs.FromGRPC(err)
}

func toKind(kind snapshotsapi.Kind) snapshots.Kind {
	if kind == snapshotsapi.Kind_ACTIVE {
		return snapshots.KindActive
	}
	if kind == snapshotsapi.Kind_VIEW {
		return snapshots.KindView
	}
	return snapshots.KindCommitted
}

func toInfo(info *snapshotsapi.Info) snapshots.Info {
	return snapshots.Info{
		Name:    info.Name,
		Parent:  info.Parent,
		Kind:    toKind(info.Kind),
		Created: protobuf.FromTimestamp(info.CreatedAt),
		Updated: protobuf.FromTimestamp(info.UpdatedAt),
		Labels:  info.Labels,
	}
}

func toUsage(resp *snapshotsapi.UsageResponse) snapshots.Usage {
	return snapshots.Usage{
		Inodes: resp.Inodes,
		Size:   resp.Size,
	}
}

func toMounts(mm []*types.Mount) []mount.Mount {
	mounts := make([]mount.Mount, len(mm))
	for i, m := range mm {
		mounts[i] = mount.Mount{
			Type:    m.Type,
			Source:  m.Source,
			Target:  m.Target,
			Options: m.Options,
		}
	}
	return mounts
}

func fromKind(kind snapshots.Kind) snapshotsapi.Kind {
	if kind == snapshots.KindActive {
		return snapshotsapi.Kind_ACTIVE
	}
	if kind == snapshots.KindView {
		return snapshotsapi.Kind_VIEW
	}
	return snapshotsapi.Kind_COMMITTED
}

func fromInfo(info snapshots.Info) *snapshotsapi.Info {
	return &snapshotsapi.Info{
		Name:      info.Name,
		Parent:    info.Parent,
		Kind:      fromKind(info.Kind),
		CreatedAt: protobuf.ToTimestamp(info.Created),
		UpdatedAt: protobuf.ToTimestamp(info.Updated),
		Labels:    info.Labels,
	}
}
