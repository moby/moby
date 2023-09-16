//go:build !windows

package containerd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/docker/docker/pkg/idtools"
)

func (i *ImageService) remapSnapshot(ctx context.Context, snapshotter snapshots.Snapshotter, id string, parentSnapshot string) error {
	rootPair := i.idMapping.RootPair()
	usernsID := fmt.Sprintf("%s-%d-%d", parentSnapshot, rootPair.UID, rootPair.GID)
	remappedID := usernsID + remapSuffix

	// If the remapped snapshot already exist we only need to prepare the new snapshot
	if _, err := snapshotter.Stat(ctx, usernsID); err == nil {
		_, err = snapshotter.Prepare(ctx, id, usernsID)
		return err
	}

	mounts, err := snapshotter.Prepare(ctx, remappedID, parentSnapshot)
	if err != nil {
		return err
	}

	if err := i.remapRootFS(ctx, mounts); err != nil {
		if rmErr := snapshotter.Remove(ctx, usernsID); rmErr != nil {
			log.G(ctx).WithError(rmErr).Warn("failed to remove snapshot after remap error")
		}
		return err
	}

	if err := snapshotter.Commit(ctx, usernsID, remappedID); err != nil {
		return err
	}

	_, err = snapshotter.Prepare(ctx, id, usernsID)
	return err
}

func (i *ImageService) remapRootFS(ctx context.Context, mounts []mount.Mount) error {
	return mount.WithTempMount(ctx, mounts, func(root string) error {
		return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			stat := info.Sys().(*syscall.Stat_t)
			if stat == nil {
				return fmt.Errorf("cannot get underlying data for %s", path)
			}

			ids, err := i.idMapping.ToHost(idtools.Identity{UID: int(stat.Uid), GID: int(stat.Gid)})
			if err != nil {
				return err
			}

			return os.Lchown(path, ids.UID, ids.GID)
		})
	})
}
