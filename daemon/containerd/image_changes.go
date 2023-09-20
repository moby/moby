package containerd

import (
	"context"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/archive"
)

func (i *ImageService) Changes(ctx context.Context, container *container.Container) ([]archive.Change, error) {
	snapshotter := i.client.SnapshotService(container.Driver)
	info, err := snapshotter.Stat(ctx, container.ID)
	if err != nil {
		return nil, err
	}

	imageMounts, _ := snapshotter.View(ctx, container.ID+"-parent-view", info.Parent)

	defer func() {
		if err := snapshotter.Remove(ctx, container.ID+"-parent-view"); err != nil {
			log.G(ctx).WithError(err).Warn("error removing the parent view snapshot")
		}
	}()

	var changes []archive.Change
	err = i.PerformWithBaseFS(ctx, container, func(containerRoot string) error {
		return mount.WithReadonlyTempMount(ctx, imageMounts, func(imageRoot string) error {
			changes, err = archive.ChangesDirs(containerRoot, imageRoot)
			return err
		})
	})

	return changes, err
}
