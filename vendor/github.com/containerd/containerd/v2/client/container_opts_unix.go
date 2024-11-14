//go:build !windows

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

package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/internal/userns"

	"github.com/containerd/errdefs"
	"github.com/opencontainers/image-spec/identity"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// WithRemappedSnapshot creates a new snapshot and remaps the uid/gid for the
// filesystem to be used by a container with user namespaces
func WithRemappedSnapshot(id string, i Image, uid, gid uint32) NewContainerOpts {
	uidmaps := []specs.LinuxIDMapping{{ContainerID: 0, HostID: uid, Size: 65536}}
	gidmaps := []specs.LinuxIDMapping{{ContainerID: 0, HostID: gid, Size: 65536}}
	return withRemappedSnapshotBase(id, i, uidmaps, gidmaps, false)
}

// WithUserNSRemappedSnapshot creates a new snapshot and remaps the uid/gid for the
// filesystem to be used by a container with user namespaces
func WithUserNSRemappedSnapshot(id string, i Image, uidmaps, gidmaps []specs.LinuxIDMapping) NewContainerOpts {
	return withRemappedSnapshotBase(id, i, uidmaps, gidmaps, false)
}

// WithRemappedSnapshotView is similar to WithRemappedSnapshot but rootfs is mounted as read-only.
func WithRemappedSnapshotView(id string, i Image, uid, gid uint32) NewContainerOpts {
	uidmaps := []specs.LinuxIDMapping{{ContainerID: 0, HostID: uid, Size: 65536}}
	gidmaps := []specs.LinuxIDMapping{{ContainerID: 0, HostID: gid, Size: 65536}}
	return withRemappedSnapshotBase(id, i, uidmaps, gidmaps, true)
}

// WithUserNSRemappedSnapshotView is similar to WithUserNSRemappedSnapshot but rootfs is mounted as read-only.
func WithUserNSRemappedSnapshotView(id string, i Image, uidmaps, gidmaps []specs.LinuxIDMapping) NewContainerOpts {
	return withRemappedSnapshotBase(id, i, uidmaps, gidmaps, true)
}

func withRemappedSnapshotBase(id string, i Image, uidmaps, gidmaps []specs.LinuxIDMapping, readonly bool) NewContainerOpts {
	return func(ctx context.Context, client *Client, c *containers.Container) error {
		diffIDs, err := i.(*image).i.RootFS(ctx, client.ContentStore(), client.platform)
		if err != nil {
			return err
		}

		rsn := remappedSnapshot{
			Parent: identity.ChainID(diffIDs).String(),
			IDMap:  userns.IDMap{UidMap: uidmaps, GidMap: gidmaps},
		}
		usernsID, err := rsn.ID()
		if err != nil {
			return fmt.Errorf("failed to remap snapshot: %w", err)
		}

		c.Snapshotter, err = client.resolveSnapshotterName(ctx, c.Snapshotter)
		if err != nil {
			return err
		}
		snapshotter, err := client.getSnapshotter(ctx, c.Snapshotter)
		if err != nil {
			return err
		}
		if _, err := snapshotter.Stat(ctx, usernsID); err == nil {
			if _, err := snapshotter.Prepare(ctx, id, usernsID); err == nil {
				c.SnapshotKey = id
				c.Image = i.Name()
				return nil
			} else if !errdefs.IsNotFound(err) {
				return err
			}
		}
		mounts, err := snapshotter.Prepare(ctx, usernsID+"-remap", rsn.Parent)
		if err != nil {
			return err
		}
		if err := remapRootFS(ctx, mounts, rsn.IDMap); err != nil {
			snapshotter.Remove(ctx, usernsID)
			return err
		}
		if err := snapshotter.Commit(ctx, usernsID, usernsID+"-remap"); err != nil {
			return err
		}
		if readonly {
			_, err = snapshotter.View(ctx, id, usernsID)
		} else {
			_, err = snapshotter.Prepare(ctx, id, usernsID)
		}
		if err != nil {
			return err
		}
		c.SnapshotKey = id
		c.Image = i.Name()
		return nil
	}
}

func remapRootFS(ctx context.Context, mounts []mount.Mount, idMap userns.IDMap) error {
	return mount.WithTempMount(ctx, mounts, func(root string) error {
		return filepath.Walk(root, chown(root, idMap))
	})
}

func chown(root string, idMap userns.IDMap) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		stat := info.Sys().(*syscall.Stat_t)
		h, cerr := idMap.ToHost(userns.User{Uid: stat.Uid, Gid: stat.Gid})
		if cerr != nil {
			return cerr
		}
		// be sure the lchown the path as to not de-reference the symlink to a host file
		if cerr = os.Lchown(path, int(h.Uid), int(h.Gid)); cerr != nil {
			return cerr
		}
		// we must retain special permissions such as setuid, setgid and sticky bits
		if mode := info.Mode(); mode&os.ModeSymlink == 0 && mode&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
			return os.Chmod(path, mode)
		}
		return nil
	}
}
