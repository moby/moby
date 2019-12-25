package containerd

import (
	"context"

	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/snapshots"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/snapshot"
	"github.com/pkg/errors"
)

func NewSnapshotter(name string, snapshotter snapshots.Snapshotter, ns string, idmap *idtools.IdentityMapping) snapshot.Snapshotter {
	return snapshot.FromContainerdSnapshotter(name, &nsSnapshotter{ns, snapshotter}, idmap)
}

func NSSnapshotter(ns string, snapshotter snapshots.Snapshotter) snapshots.Snapshotter {
	return &nsSnapshotter{ns: ns, Snapshotter: snapshotter}
}

type nsSnapshotter struct {
	ns string
	snapshots.Snapshotter
}

func (s *nsSnapshotter) Stat(ctx context.Context, key string) (snapshots.Info, error) {
	ctx = namespaces.WithNamespace(ctx, s.ns)
	return s.Snapshotter.Stat(ctx, key)
}

func (s *nsSnapshotter) Update(ctx context.Context, info snapshots.Info, fieldpaths ...string) (snapshots.Info, error) {
	ctx = namespaces.WithNamespace(ctx, s.ns)
	return s.Snapshotter.Update(ctx, info, fieldpaths...)
}

func (s *nsSnapshotter) Usage(ctx context.Context, key string) (snapshots.Usage, error) {
	ctx = namespaces.WithNamespace(ctx, s.ns)
	return s.Snapshotter.Usage(ctx, key)
}
func (s *nsSnapshotter) Mounts(ctx context.Context, key string) ([]mount.Mount, error) {
	ctx = namespaces.WithNamespace(ctx, s.ns)
	return s.Snapshotter.Mounts(ctx, key)
}
func (s *nsSnapshotter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	ctx = namespaces.WithNamespace(ctx, s.ns)
	return s.Snapshotter.Prepare(ctx, key, parent, opts...)
}
func (s *nsSnapshotter) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	ctx = namespaces.WithNamespace(ctx, s.ns)
	return s.Snapshotter.View(ctx, key, parent, opts...)
}
func (s *nsSnapshotter) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) error {
	ctx = namespaces.WithNamespace(ctx, s.ns)
	return s.Snapshotter.Commit(ctx, name, key, opts...)
}
func (s *nsSnapshotter) Remove(ctx context.Context, key string) error {
	return errors.Errorf("calling snapshotter.Remove is forbidden")
}
func (s *nsSnapshotter) Walk(ctx context.Context, fn func(context.Context, snapshots.Info) error) error {
	ctx = namespaces.WithNamespace(ctx, s.ns)
	return s.Snapshotter.Walk(ctx, fn)
}
