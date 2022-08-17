package containerd

import (
	"context"
	"fmt"
	"os"

	"github.com/containerd/containerd/mount"
	"github.com/docker/docker/container"
	"github.com/sirupsen/logrus"
)

// Mount mounts and sets container base filesystem
func (i *ImageService) Mount(ctx context.Context, container *container.Container) error {
	snapshotter := i.client.SnapshotService(i.snapshotter)
	mounts, err := snapshotter.Mounts(ctx, container.ID)
	if err != nil {
		return err
	}

	tempMountLocation := os.TempDir()

	root, err := os.MkdirTemp(tempMountLocation, "rootfs-mount")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	if err := mount.All(mounts, root); err != nil {
		return fmt.Errorf("failed to mount %s: %w", root, err)
	}

	container.BaseFS = root
	return nil
}

// Unmount unmounts the container base filesystem
func (i *ImageService) Unmount(ctx context.Context, container *container.Container) error {
	root := container.BaseFS

	if err := mount.UnmountAll(root, 0); err != nil {
		return fmt.Errorf("failed to unmount %s: %w", root, err)
	}

	if err := os.Remove(root); err != nil {
		logrus.WithError(err).WithField("dir", root).Error("failed to remove mount temp dir")
	}

	container.BaseFS = ""

	return nil
}
