//go:build !windows

package containerd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/continuity/fs"
	"github.com/containerd/continuity/sysx"
	"github.com/docker/docker/pkg/idtools"
)

const (
	// Values based on linux/include/uapi/linux/capability.h
	xattrCapsSz2    = 20
	versionOffset   = 3
	vfsCapRevision2 = 2
	vfsCapRevision3 = 3
	remapSuffix     = "-remap"
)

func (i *ImageService) remapSnapshot(ctx context.Context, snapshotter snapshots.Snapshotter, id string, parentSnapshot string) error {
	_, err := snapshotter.Prepare(ctx, id, parentSnapshot)
	if err != nil {
		return err
	}
	mounts, err := snapshotter.Mounts(ctx, id)
	if err != nil {
		return err
	}

	if err := i.remapRootFS(ctx, mounts); err != nil {
		return err
	}

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

			return chownWithCaps(path, ids.UID, ids.GID)
		})
	})
}

func (i *ImageService) copyAndUnremapRootFS(ctx context.Context, dst, src []mount.Mount) error {
	return mount.WithTempMount(ctx, src, func(source string) error {
		return mount.WithTempMount(ctx, dst, func(root string) error {
			// TODO: Update CopyDir to support remap directly
			if err := fs.CopyDir(root, source); err != nil {
				return fmt.Errorf("failed to copy: %w", err)
			}

			return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				stat := info.Sys().(*syscall.Stat_t)
				if stat == nil {
					return fmt.Errorf("cannot get underlying data for %s", path)
				}

				uid, gid, err := i.idMapping.ToContainer(idtools.Identity{UID: int(stat.Uid), GID: int(stat.Gid)})
				if err != nil {
					return err
				}

				return chownWithCaps(path, uid, gid)
			})
		})
	})
}

func (i *ImageService) unremapRootFS(ctx context.Context, mounts []mount.Mount) error {
	return mount.WithTempMount(ctx, mounts, func(root string) error {
		return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			stat := info.Sys().(*syscall.Stat_t)
			if stat == nil {
				return fmt.Errorf("cannot get underlying data for %s", path)
			}

			uid, gid, err := i.idMapping.ToContainer(idtools.Identity{UID: int(stat.Uid), GID: int(stat.Gid)})
			if err != nil {
				return err
			}

			return chownWithCaps(path, uid, gid)
		})
	})
}

// chownWithCaps will chown path and preserve the extended attributes.
// chowning a file will remove the capabilities, so we need to first get all of
// them, chown the file, and then set the extended attributes
func chownWithCaps(path string, uid int, gid int) error {
	xattrKeys, err := sysx.LListxattr(path)
	if err != nil {
		return err
	}

	xattrs := make(map[string][]byte, len(xattrKeys))

	for _, xattr := range xattrKeys {
		data, err := sysx.LGetxattr(path, xattr)
		if err != nil {
			return err
		}
		xattrs[xattr] = data
	}

	if err := os.Lchown(path, uid, gid); err != nil {
		return err
	}

	for xattrKey, xattrValue := range xattrs {
		length := len(xattrValue)
		// make sure the capabilities are version 2,
		// capabilities version 3 also store the root uid of the namespace,
		// we don't want this when we are in userns-remap mode
		// see: https://github.com/moby/moby/pull/41724
		if xattrKey == "security.capability" && xattrValue[versionOffset] == vfsCapRevision3 {
			xattrValue[versionOffset] = vfsCapRevision2
			length = xattrCapsSz2
		}
		if err := sysx.LSetxattr(path, xattrKey, xattrValue[:length], 0); err != nil {
			return err
		}
	}

	return nil
}
