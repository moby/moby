package images

import (
	"context"
	"fmt"
	"runtime"

	"github.com/containerd/log"
	"github.com/docker/docker/container"
)

// Mount sets container.BaseFS
// (is it not set coming in? why is it unset?)
func (i *ImageService) Mount(ctx context.Context, container *container.Container) error {
	rwLayer, err := i.layerStore.GetRWLayer(container.ID)
	if err != nil {
		return fmt.Errorf("RWLayer of container "+container.ID+" is unexpectedly nil: %w", err)
	}
	dir, err := rwLayer.Mount(container.GetMountLabel())
	if err != nil {
		return err
	}
	log.G(ctx).WithField("container", container.ID).Debugf("container mounted via layerStore: %v", dir)

	if container.BaseFS != "" && container.BaseFS != dir {
		// The mount path reported by the graph driver should always be trusted on Windows, since the
		// volume path for a given mounted layer may change over time.  This should only be an error
		// on non-Windows operating systems.
		if runtime.GOOS != "windows" {
			i.Unmount(ctx, container)
			return fmt.Errorf("Error: driver %s is returning inconsistent paths for container %s ('%s' then '%s')",
				i.StorageDriver(), container.ID, container.BaseFS, dir)
		}
	}
	container.BaseFS = dir // TODO: combine these fields
	return nil
}

// Unmount unsets the container base filesystem
func (i *ImageService) Unmount(ctx context.Context, container *container.Container) error {
	rwLayer, err := i.layerStore.GetRWLayer(container.ID)
	if err != nil {
		return fmt.Errorf("RWLayer of container "+container.ID+" is unexpectedly nil: %w", err)
	}
	if err := rwLayer.Unmount(); err != nil {
		log.G(ctx).WithField("container", container.ID).WithError(err).Error("error unmounting container")
		return err
	}

	return nil
}
