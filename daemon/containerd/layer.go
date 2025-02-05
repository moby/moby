package containerd

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/log"
	"github.com/docker/docker/daemon/snapshotter"
)

type snapshotLayer struct {
	readonly        bool
	id              string
	snapshotter     snapshots.Snapshotter
	snapshotterName string
	refCountMounter snapshotter.Mounter
	root            string
	lease           leases.Lease
	mut             sync.Mutex
}

func (l *snapshotLayer) mounts(ctx context.Context) ([]mount.Mount, error) {
	return l.snapshotter.Mounts(ctx, l.id)
}

func (l *snapshotLayer) Writable() bool {
	return !l.readonly
}

func (l *snapshotLayer) Mount(ctx context.Context, mountLabel string) (string, error) {
	l.mut.Lock()
	defer l.mut.Unlock()

	// TODO: Investigate how we can handle mountLabel
	_ = mountLabel
	mounts, err := l.mounts(ctx)
	if err != nil {
		return "", err
	}

	if l.readonly {
		mounts = readonlyMounts(mounts)
	}

	var root string
	if root, err = l.refCountMounter.Mount(mounts, l.id); err != nil {
		return "", fmt.Errorf("failed to mount %s: %w", root, err)
	}
	l.root = root

	log.G(ctx).WithFields(log.Fields{"container": l.id, "root": root, "snapshotter": l.snapshotterName}).Debug("container mounted via snapshotter")
	return root, nil
}

func (l *snapshotLayer) Unmount(ctx context.Context) error {
	l.mut.Lock()
	defer l.mut.Unlock()

	if l.root == "" {
		target, err := l.refCountMounter.Mounted(l.id)
		if err != nil {
			log.G(ctx).WithField("id", l.id).Warn("failed to determine if container is already mounted")
		}
		if target == "" {
			return errors.New("layer not mounted")
		}
		l.root = target
	}

	if err := l.refCountMounter.Unmount(l.root); err != nil {
		log.G(ctx).WithField("container", l.id).WithError(err).Error("error unmounting container")
		return fmt.Errorf("failed to unmount %s: %w", l.root, err)
	}

	return nil
}

func (l *snapshotLayer) Metadata() (map[string]string, error) {
	return nil, nil
}
