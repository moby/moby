package containerd

import (
	"context"
	"fmt"

	"github.com/containerd/containerd"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/platforms"
	"github.com/docker/docker/errdefs"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// PrepareSnapshot prepares a snapshot from a parent image for a container
func (i *ImageService) PrepareSnapshot(ctx context.Context, id string, parentImage string, platform *ocispec.Platform, setupInit func(string) error) error {
	var parentSnapshot string
	if parentImage != "" {
		img, err := i.resolveImage(ctx, parentImage)
		if err != nil {
			return err
		}

		cs := i.content

		matcher := matchAllWithPreference(platforms.Default())
		if platform != nil {
			matcher = platforms.Only(*platform)
		}

		platformImg := containerd.NewImageWithPlatform(i.client, img, matcher)
		unpacked, err := platformImg.IsUnpacked(ctx, i.snapshotter)
		if err != nil {
			return err
		}

		if !unpacked {
			if err := platformImg.Unpack(ctx, i.snapshotter); err != nil {
				return err
			}
		}

		desc, err := containerdimages.Config(ctx, cs, img.Target, matcher)
		if err != nil {
			return err
		}

		diffIDs, err := containerdimages.RootFS(ctx, cs, desc)
		if err != nil {
			return err
		}

		parentSnapshot = identity.ChainID(diffIDs).String()
	}

	ls := i.client.LeasesService()
	lease, err := ls.Create(ctx, leases.WithID(id))
	if err != nil {
		return err
	}
	ctx = leases.WithLease(ctx, lease.ID)

	snapshotter := i.client.SnapshotService(i.StorageDriver())

	if err := i.prepareInitLayer(ctx, id, parentSnapshot, setupInit); err != nil {
		return err
	}

	if !i.idMapping.Empty() {
		return i.remapSnapshot(ctx, snapshotter, id, id+"-init")
	}

	_, err = snapshotter.Prepare(ctx, id, id+"-init")
	return err
}

func (i *ImageService) prepareInitLayer(ctx context.Context, id string, parent string, setupInit func(string) error) error {
	snapshotter := i.client.SnapshotService(i.StorageDriver())

	mounts, err := snapshotter.Prepare(ctx, id+"-init-key", parent)
	if err != nil {
		return err
	}

	if setupInit != nil {
		if err := mount.WithTempMount(ctx, mounts, func(root string) error {
			return setupInit(root)
		}); err != nil {
			return err
		}
	}

	return snapshotter.Commit(ctx, id+"-init", id+"-init-key")
}

// calculateSnapshotParentUsage returns the usage of all ancestors of the
// provided snapshot. It doesn't include the size of the snapshot itself.
func calculateSnapshotParentUsage(ctx context.Context, snapshotter snapshots.Snapshotter, snapshotID string) (snapshots.Usage, error) {
	info, err := snapshotter.Stat(ctx, snapshotID)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return snapshots.Usage{}, errdefs.NotFound(err)
		}
		return snapshots.Usage{}, errdefs.System(errors.Wrapf(err, "snapshotter.Stat failed for %s", snapshotID))
	}
	if info.Parent == "" {
		return snapshots.Usage{}, errdefs.NotFound(fmt.Errorf("snapshot %s has no parent", snapshotID))
	}

	return calculateSnapshotTotalUsage(ctx, snapshotter, info.Parent)
}

// calculateSnapshotTotalUsage returns the total usage of that snapshot
// including all of its ancestors.
func calculateSnapshotTotalUsage(ctx context.Context, snapshotter snapshots.Snapshotter, snapshotID string) (snapshots.Usage, error) {
	var total snapshots.Usage
	next := snapshotID

	for next != "" {
		usage, err := snapshotter.Usage(ctx, next)
		if err != nil {
			if cerrdefs.IsNotFound(err) {
				return total, errdefs.NotFound(errors.Wrapf(err, "non-existing ancestor of %s", snapshotID))
			}
			return total, errdefs.System(errors.Wrapf(err, "snapshotter.Usage failed for %s", next))
		}
		total.Size += usage.Size
		total.Inodes += usage.Inodes

		info, err := snapshotter.Stat(ctx, next)
		if err != nil {
			return total, errdefs.System(errors.Wrapf(err, "snapshotter.Stat failed for %s", next))
		}
		next = info.Parent
	}
	return total, nil
}
