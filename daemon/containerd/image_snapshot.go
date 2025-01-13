package containerd

import (
	"context"
	"fmt"

	c8dimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// CreateLayer creates a new layer for a container.
// TODO(vvoland): Decouple from container
func (i *ImageService) CreateLayer(ctr *container.Container, initFunc layer.MountInit) (container.Layer, error) {
	ctx := context.TODO()

	var parentSnapshot string
	if ctr.ImageManifest != nil {
		s, err := i.getImageSnapshot(ctx, *ctr.ImageManifest)
		if err != nil {
			return nil, err
		}
		parentSnapshot = s
	}

	id := ctr.ID

	// TODO: Consider a better way to do this. It is better to have a container directly
	// reference a snapshot, however, that is not done today because a container may
	// removed and recreated with nothing holding the snapshot in between. Consider
	// removing this lease and only temporarily holding a lease on re-create, using
	// non-expiring leases introduces the possibility of leaking resources.
	ls := i.client.LeasesService()
	lease, err := ls.Create(ctx, leases.WithID(id))
	if err != nil {
		return nil, err
	}
	ctx = leases.WithLease(ctx, lease.ID)

	if err := i.prepareInitLayer(ctx, id, parentSnapshot, initFunc); err != nil {
		return nil, err
	}

	sn := i.client.SnapshotService(i.StorageDriver())
	if !i.idMapping.Empty() {
		err = i.remapSnapshot(ctx, sn, id, id+"-init")
	} else {
		_, err = sn.Prepare(ctx, id, id+"-init")
	}

	if err != nil {
		return nil, err
	}

	return &snapshotLayer{
		id:              id,
		snapshotterName: i.StorageDriver(),
		snapshotter:     sn,
		refCountMounter: i.refCountMounter,
		lease:           lease,
	}, nil
}

func (i *ImageService) GetImageLayer(ctx context.Context, img *image.Image) (container.Layer, error) {
	if img.Details.ManifestDescriptor == nil {
		return nil, fmt.Errorf("no manifest descriptor found for image %s", img.ID)
	}

	snapshotID, err := i.getImageSnapshot(ctx, *img.Details.ManifestDescriptor)
	if err != nil {
		return nil, err
	}

	snapshotterName := i.snapshotter
	leaseId := "imagelayer-" + snapshotID
	lm := i.client.LeasesService()

	lease, err := getLeaseByID(ctx, lm, leaseId)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return nil, err
		}

		lease, err = lm.Create(ctx, leases.WithID(leaseId))
		if err != nil {
			return nil, err
		}
	}

	if err := lm.AddResource(ctx, lease, leases.Resource{
		Type: "snapshots/" + snapshotterName,
		ID:   snapshotID,
	}); err != nil {
		if err := lm.Delete(ctx, lease, leases.SynchronousDelete); err != nil {
			log.G(ctx).WithError(err).Warn("failed to cleanup lease")
		}
		return nil, err
	}

	return &snapshotLayer{
		id:              snapshotID,
		readonly:        true,
		snapshotterName: i.StorageDriver(),
		snapshotter:     i.client.SnapshotService(i.StorageDriver()),
		refCountMounter: i.refCountMounter,
		lease:           lease,
	}, nil
}

func (i *ImageService) ReleaseImageLayer(ctx context.Context, l container.Layer) error {
	c8dLayer, ok := l.(*snapshotLayer)
	if !ok {
		return fmt.Errorf("invalid layer type %T", l)
	}

	ls := i.client.LeasesService()
	if err := ls.Delete(ctx, c8dLayer.lease, leases.SynchronousDelete); err != nil {
		if !cerrdefs.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (i *ImageService) getImageSnapshot(ctx context.Context, desc ocispec.Descriptor) (string, error) {
	platformImg, err := i.NewImageManifest(ctx, c8dimages.Image{Target: desc}, desc)
	if err != nil {
		return "", err
	}
	unpacked, err := platformImg.IsUnpacked(ctx, i.snapshotter)
	if err != nil {
		return "", err
	}

	if !unpacked {
		if err := platformImg.Unpack(ctx, i.snapshotter); err != nil {
			return "", err
		}
	}

	diffIDs, err := platformImg.RootFS(ctx)
	if err != nil {
		return "", err
	}

	return identity.ChainID(diffIDs).String(), nil
}

// GetLayerByID returns a layer by ID
// called from daemon.go Daemon.restore().
func (i *ImageService) GetLayerByID(cid string) (container.Layer, error) {
	ctx := context.TODO()

	sn := i.client.SnapshotService(i.StorageDriver())
	if _, err := sn.Stat(ctx, cid); err != nil {
		if !cerrdefs.IsNotFound(err) {
			return nil, fmt.Errorf("failed to stat snapshot %s: %w", cid, err)
		}
		return nil, errdefs.NotFound(fmt.Errorf("RW layer for container %s not found", cid))
	}

	lease, err := getLeaseByID(ctx, i.client.LeasesService(), cid)
	if err != nil {
		return nil, err
	}

	root, err := i.refCountMounter.Mounted(cid)
	if err != nil {
		log.G(ctx).WithField("container", cid).Warn("failed to determine if container is already mounted")
	}

	return &snapshotLayer{
		id:              cid,
		snapshotterName: i.StorageDriver(),
		snapshotter:     sn,
		refCountMounter: i.refCountMounter,
		lease:           lease,
		root:            root,
	}, nil

}

// ReleaseLayer releases a layer allowing it to be removed
// called from delete.go Daemon.cleanupContainer(), and Daemon.containerExport()
func (i *ImageService) ReleaseLayer(rwlayer container.Layer) error {
	c8dLayer, ok := rwlayer.(*snapshotLayer)
	if !ok {
		return fmt.Errorf("invalid layer type %T", rwlayer)
	}

	ls := i.client.LeasesService()
	if err := ls.Delete(context.Background(), c8dLayer.lease, leases.SynchronousDelete); err != nil {
		if !cerrdefs.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (i *ImageService) prepareInitLayer(ctx context.Context, id string, parent string, setupInit func(string) error) error {
	sn := i.client.SnapshotService(i.StorageDriver())

	mounts, err := sn.Prepare(ctx, id+"-init-key", parent)
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

	return sn.Commit(ctx, id+"-init", id+"-init-key")
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

func getLeaseByID(ctx context.Context, lm leases.Manager, id string) (leases.Lease, error) {
	lss, err := lm.List(ctx, "id=="+id)
	if err != nil {
		return leases.Lease{}, nil
	}

	switch len(lss) {
	case 0:
		return leases.Lease{}, errdefs.NotFound(errors.New("lease not found" + id))
	default:
		log.G(ctx).WithFields(log.Fields{"lease": id, "leases": lss}).Warn("multiple leases with the same id found, this should not happen")
	case 1:
	}
	return lss[0], nil
}
