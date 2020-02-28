package snapshot

import (
	"context"
	"os"
	"sync"
	"sync/atomic"

	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/docker/docker/pkg/idtools"
)

type Mountable interface {
	// ID() string
	Mount() ([]mount.Mount, func() error, error)
	IdentityMapping() *idtools.IdentityMapping
}

// Snapshotter defines interface that any snapshot implementation should satisfy
type Snapshotter interface {
	Name() string
	Mounts(ctx context.Context, key string) (Mountable, error)
	Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) error
	View(ctx context.Context, key, parent string, opts ...snapshots.Opt) (Mountable, error)

	Stat(ctx context.Context, key string) (snapshots.Info, error)
	Update(ctx context.Context, info snapshots.Info, fieldpaths ...string) (snapshots.Info, error)
	Usage(ctx context.Context, key string) (snapshots.Usage, error)
	Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) error
	Remove(ctx context.Context, key string) error
	Walk(ctx context.Context, fn snapshots.WalkFunc, filters ...string) error
	Close() error
	IdentityMapping() *idtools.IdentityMapping
}

func FromContainerdSnapshotter(name string, s snapshots.Snapshotter, idmap *idtools.IdentityMapping) Snapshotter {
	return &fromContainerd{name: name, Snapshotter: s, idmap: idmap}
}

type fromContainerd struct {
	name string
	snapshots.Snapshotter
	idmap *idtools.IdentityMapping
}

func (s *fromContainerd) Name() string {
	return s.name
}

func (s *fromContainerd) Mounts(ctx context.Context, key string) (Mountable, error) {
	mounts, err := s.Snapshotter.Mounts(ctx, key)
	if err != nil {
		return nil, err
	}
	return &staticMountable{mounts: mounts, idmap: s.idmap, id: key}, nil
}
func (s *fromContainerd) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) error {
	_, err := s.Snapshotter.Prepare(ctx, key, parent, opts...)
	return err
}
func (s *fromContainerd) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) (Mountable, error) {
	mounts, err := s.Snapshotter.View(ctx, key, parent, opts...)
	if err != nil {
		return nil, err
	}
	return &staticMountable{mounts: mounts, idmap: s.idmap, id: key}, nil
}
func (s *fromContainerd) IdentityMapping() *idtools.IdentityMapping {
	return s.idmap
}

type staticMountable struct {
	count  int32
	id     string
	mounts []mount.Mount
	idmap  *idtools.IdentityMapping
}

func (cm *staticMountable) Mount() ([]mount.Mount, func() error, error) {
	atomic.AddInt32(&cm.count, 1)
	return cm.mounts, func() error {
		if atomic.AddInt32(&cm.count, -1) < 0 {
			if v := os.Getenv("BUILDKIT_DEBUG_PANIC_ON_ERROR"); v == "1" {
				panic("release of released mount " + cm.id)
			}
		}
		return nil
	}, nil
}

func (cm *staticMountable) IdentityMapping() *idtools.IdentityMapping {
	return cm.idmap
}

// NewContainerdSnapshotter converts snapshotter to containerd snapshotter
func NewContainerdSnapshotter(s Snapshotter) (snapshots.Snapshotter, func() error) {
	cs := &containerdSnapshotter{Snapshotter: s}
	return cs, cs.release
}

type containerdSnapshotter struct {
	mu        sync.Mutex
	releasers []func() error
	Snapshotter
}

func (cs *containerdSnapshotter) release() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	var err error
	for _, f := range cs.releasers {
		if err1 := f(); err1 != nil && err == nil {
			err = err1
		}
	}
	return err
}

func (cs *containerdSnapshotter) returnMounts(mf Mountable) ([]mount.Mount, error) {
	mounts, release, err := mf.Mount()
	if err != nil {
		return nil, err
	}
	cs.mu.Lock()
	cs.releasers = append(cs.releasers, release)
	cs.mu.Unlock()
	return mounts, nil
}

func (cs *containerdSnapshotter) Mounts(ctx context.Context, key string) ([]mount.Mount, error) {
	mf, err := cs.Snapshotter.Mounts(ctx, key)
	if err != nil {
		return nil, err
	}
	return cs.returnMounts(mf)
}

func (cs *containerdSnapshotter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	if err := cs.Snapshotter.Prepare(ctx, key, parent, opts...); err != nil {
		return nil, err
	}
	return cs.Mounts(ctx, key)
}
func (cs *containerdSnapshotter) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	mf, err := cs.Snapshotter.View(ctx, key, parent, opts...)
	if err != nil {
		return nil, err
	}
	return cs.returnMounts(mf)
}
