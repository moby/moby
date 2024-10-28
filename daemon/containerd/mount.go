package containerd

import (
	"context"
	"errors"
	"fmt"

	"github.com/containerd/log"
	"github.com/docker/docker/container"
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

	log.G(ctx).WithFields(log.Fields{"container": container.ID, "root": root, "snapshotter": container.Driver}).Debug("container mounted via snapshotter")

	container.BaseFS = root
	return nil
}

// Unmount unmounts the container base filesystem
func (i *ImageService) Unmount(ctx context.Context, container *container.Container) error {
	baseFS := container.BaseFS
	if baseFS == "" {
		target, err := i.refCountMounter.Mounted(container.ID)
		if err != nil {
			log.G(ctx).WithField("containerID", container.ID).Warn("failed to determine if container is already mounted")
		}
		if target == "" {
			return errors.New("BaseFS is empty")
		}
		baseFS = target
	}

	if err := i.refCountMounter.Unmount(baseFS); err != nil {
		log.G(ctx).WithField("container", container.ID).WithError(err).Error("error unmounting container")
		return fmt.Errorf("failed to unmount %s: %w", baseFS, err)
	}

	return nil
}
