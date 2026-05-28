package containerd

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/log"
	"github.com/moby/go-archive"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/internal/stringid"
)

func (i *ImageService) Changes(ctx context.Context, ctr *container.Container) ([]archive.Change, error) {
	rwl := ctr.RWLayer
	if rwl == nil {
		return nil, fmt.Errorf("RWLayer is unexpectedly nil for container %s", ctr.ID)
	}
	snapshotter := i.client.SnapshotService(ctr.Driver)
	info, err := snapshotter.Stat(ctx, ctr.ID)
	if err != nil {
		return nil, err
	}

	id := stringid.GenerateRandomID()
	parentViewKey := ctr.ID + "-parent-view-" + id
	imageMounts, _ := snapshotter.View(ctx, parentViewKey, info.Parent)

	defer func() {
		if err := snapshotter.Remove(ctx, parentViewKey); err != nil {
			log.G(ctx).WithError(err).Warn("error removing the parent view snapshot")
		}
	}()

	containerRoot, err := rwl.Mount(ctr.GetMountLabel())
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rwl.Unmount(); err != nil {
			log.G(ctx).WithFields(log.Fields{"error": err, "container": ctr.ID}).Warn("Failed to unmount container RWLayer after export")
		}
	}()
	var changes []archive.Change
	err = mount.WithReadonlyTempMount(ctx, imageMounts, func(imageRoot string) error {
		changes, err = archive.ChangesDirs(containerRoot, imageRoot)
		return err
	})
	return changes, err
}
