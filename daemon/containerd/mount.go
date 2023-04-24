package containerd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/containerd/containerd/mount"
	"github.com/docker/docker/container"
	"github.com/sirupsen/logrus"
)

const rootfsMountSuffix = "_rootfs-mount"

// Mount mounts the container filesystem in a temporary location, use defer imageService.Unmount
// to unmount the filesystem when calling this
func (i *ImageService) Mount(ctx context.Context, container *container.Container) error {
	snapshotter := i.client.SnapshotService(container.Driver)
	mounts, err := snapshotter.Mounts(ctx, container.ID)
	if err != nil {
		return err
	}

	root := ""
	if container.State != nil && !container.State.Running && container.Pid == 0 {
		// The temporary location will be under /var/lib/docker/... because
		// we set the `TMPDIR`
		var err error
		root, err = os.MkdirTemp("", container.ID+rootfsMountSuffix)
		if err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}

		if err := mount.All(mounts, root); err != nil {
			return fmt.Errorf("failed to mount %s: %w", root, err)
		}
	} else {
		root = fmt.Sprintf("/proc/%d/root", container.Pid)
	}

	container.BaseFS = root
	return nil
}

// Unmount unmounts the container base filesystem
func (i *ImageService) Unmount(ctx context.Context, container *container.Container) error {
	root := container.BaseFS

	if strings.HasSuffix(root, rootfsMountSuffix) {
		if err := mount.UnmountAll(root, 0); err != nil {
			return fmt.Errorf("failed to unmount %s: %w", root, err)
		}

		if err := os.Remove(root); err != nil {
			logrus.WithError(err).WithField("dir", root).Error("failed to remove mount temp dir")
		}
	}

	container.BaseFS = ""

	return nil
}
