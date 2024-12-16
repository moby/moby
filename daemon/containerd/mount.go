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
func (i *ImageService) Mount(ctx context.Context, ctr *container.Container) error {
	if ctr.RWLayer == nil {
		return errors.New("RWLayer of container " + ctr.ID + " is unexpectedly nil")
	}

	root, err := ctr.RWLayer.Mount(ctr.GetMountLabel())
	if err != nil {
		return fmt.Errorf("failed to mount container %s: %w", ctr.ID, err)
	}

	log.G(ctx).WithFields(log.Fields{"ctr": ctr.ID, "root": root, "snapshotter": ctr.Driver}).Debug("ctr mounted via snapshotter")

	ctr.BaseFS = root
	return nil
}

// Unmount unmounts the container base filesystem
func (i *ImageService) Unmount(ctx context.Context, ctr *container.Container) error {
	if ctr.RWLayer == nil {
		return errors.New("RWLayer of container " + ctr.ID + " is unexpectedly nil")
	}

	if err := ctr.RWLayer.Unmount(); err != nil {
		return fmt.Errorf("failed to unmount container %s: %w", ctr.ID, err)
	}
	return nil
}
