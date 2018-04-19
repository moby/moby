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

package containerd

import (
	"context"
	"io"

	snapshotsapi "github.com/containerd/containerd/api/services/snapshots/v1"
	"github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	protobuftypes "github.com/gogo/protobuf/types"
)

// NewSnapshotterFromClient returns a new Snapshotter which communicates
// over a GRPC connection.
func NewSnapshotterFromClient(client snapshotsapi.SnapshotsClient, snapshotterName string) snapshots.Snapshotter {
	return &remoteSnapshotter{
		client:          client,
		snapshotterName: snapshotterName,
	}
}

type remoteSnapshotter struct {
	client          snapshotsapi.SnapshotsClient
	snapshotterName string
}

func (r *remoteSnapshotter) Stat(ctx context.Context, key string) (snapshots.Info, error) {
	resp, err := r.client.Stat(ctx,
		&snapshotsapi.StatSnapshotRequest{
			Snapshotter: r.snapshotterName,
			Key:         key,
		})
	if err != nil {
		return snapshots.Info{}, errdefs.FromGRPC(err)
	}
	return toInfo(resp.Info), nil
}

func (r *remoteSnapshotter) Update(ctx context.Context, info snapshots.Info, fieldpaths ...string) (snapshots.Info, error) {
	resp, err := r.client.Update(ctx,
		&snapshotsapi.UpdateSnapshotRequest{
			Snapshotter: r.snapshotterName,
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

func (r *remoteSnapshotter) Usage(ctx context.Context, key string) (snapshots.Usage, error) {
	resp, err := r.client.Usage(ctx, &snapshotsapi.UsageRequest{
		Snapshotter: r.snapshotterName,
		Key:         key,
	})
	if err != nil {
		return snapshots.Usage{}, errdefs.FromGRPC(err)
	}
	return toUsage(resp), nil
}

func (r *remoteSnapshotter) Mounts(ctx context.Context, key string) ([]mount.Mount, error) {
	resp, err := r.client.Mounts(ctx, &snapshotsapi.MountsRequest{
		Snapshotter: r.snapshotterName,
		Key:         key,
	})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}
	return toMounts(resp.Mounts), nil
}

func (r *remoteSnapshotter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	var local snapshots.Info
	for _, opt := range opts {
		if err := opt(&local); err != nil {
			return nil, err
		}
	}
	resp, err := r.client.Prepare(ctx, &snapshotsapi.PrepareSnapshotRequest{
		Snapshotter: r.snapshotterName,
		Key:         key,
		Parent:      parent,
		Labels:      local.Labels,
	})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}
	return toMounts(resp.Mounts), nil
}

func (r *remoteSnapshotter) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	var local snapshots.Info
	for _, opt := range opts {
		if err := opt(&local); err != nil {
			return nil, err
		}
	}
	resp, err := r.client.View(ctx, &snapshotsapi.ViewSnapshotRequest{
		Snapshotter: r.snapshotterName,
		Key:         key,
		Parent:      parent,
		Labels:      local.Labels,
	})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}
	return toMounts(resp.Mounts), nil
}

func (r *remoteSnapshotter) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) error {
	var local snapshots.Info
	for _, opt := range opts {
		if err := opt(&local); err != nil {
			return err
		}
	}
	_, err := r.client.Commit(ctx, &snapshotsapi.CommitSnapshotRequest{
		Snapshotter: r.snapshotterName,
		Name:        name,
		Key:         key,
		Labels:      local.Labels,
	})
	return errdefs.FromGRPC(err)
}

func (r *remoteSnapshotter) Remove(ctx context.Context, key string) error {
	_, err := r.client.Remove(ctx, &snapshotsapi.RemoveSnapshotRequest{
		Snapshotter: r.snapshotterName,
		Key:         key,
	})
	return errdefs.FromGRPC(err)
}

func (r *remoteSnapshotter) Walk(ctx context.Context, fn func(context.Context, snapshots.Info) error) error {
	sc, err := r.client.List(ctx, &snapshotsapi.ListSnapshotsRequest{
		Snapshotter: r.snapshotterName,
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

func (r *remoteSnapshotter) Close() error {
	return nil
}

func toKind(kind snapshotsapi.Kind) snapshots.Kind {
	if kind == snapshotsapi.KindActive {
		return snapshots.KindActive
	}
	if kind == snapshotsapi.KindView {
		return snapshots.KindView
	}
	return snapshots.KindCommitted
}

func toInfo(info snapshotsapi.Info) snapshots.Info {
	return snapshots.Info{
		Name:    info.Name,
		Parent:  info.Parent,
		Kind:    toKind(info.Kind),
		Created: info.CreatedAt,
		Updated: info.UpdatedAt,
		Labels:  info.Labels,
	}
}

func toUsage(resp *snapshotsapi.UsageResponse) snapshots.Usage {
	return snapshots.Usage{
		Inodes: resp.Inodes,
		Size:   resp.Size_,
	}
}

func toMounts(mm []*types.Mount) []mount.Mount {
	mounts := make([]mount.Mount, len(mm))
	for i, m := range mm {
		mounts[i] = mount.Mount{
			Type:    m.Type,
			Source:  m.Source,
			Options: m.Options,
		}
	}
	return mounts
}

func fromKind(kind snapshots.Kind) snapshotsapi.Kind {
	if kind == snapshots.KindActive {
		return snapshotsapi.KindActive
	}
	if kind == snapshots.KindView {
		return snapshotsapi.KindView
	}
	return snapshotsapi.KindCommitted
}

func fromInfo(info snapshots.Info) snapshotsapi.Info {
	return snapshotsapi.Info{
		Name:      info.Name,
		Parent:    info.Parent,
		Kind:      fromKind(info.Kind),
		CreatedAt: info.Created,
		UpdatedAt: info.Updated,
		Labels:    info.Labels,
	}
}
