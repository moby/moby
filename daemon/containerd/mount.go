package containerd

import (
	"context"
	"fmt"

	"github.com/docker/docker/container"
	"github.com/sirupsen/logrus"
)

// Mount mounts the container filesystem in a temporary location, use defer imageService.Unmount
// to unmount the filesystem when calling this
func (i *ImageService) Mount(ctx context.Context, container *container.Container) error {
	snapshotter := i.client.SnapshotService(container.Driver)
	mounts, err := snapshotter.Mounts(ctx, container.ID)
	if err != nil {
		return err
	}

	var root string
	if root, err = i.refCountMounter.Mount(mounts, container.ID); err != nil {
		return fmt.Errorf("failed to mount %s: %w", root, err)
	}

	logrus.WithField("container", container.ID).Debugf("container mounted via snapshotter: %v", root)

	container.BaseFS = root
	return nil
}

// Unmount unmounts the container base filesystem
func (i *ImageService) Unmount(ctx context.Context, container *container.Container) error {
	root := container.BaseFS

	if err := i.refCountMounter.Unmount(root); err != nil {
		logrus.WithField("container", container.ID).WithError(err).Error("error unmounting container")
		return fmt.Errorf("failed to unmount %s: %w", root, err)
	}

	return nil
}
