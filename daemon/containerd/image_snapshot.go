package containerd

import (
	"context"

	"github.com/containerd/containerd"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/platforms"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// PrepareSnapshot prepares a snapshot from a parent image for a container
func (i *ImageService) PrepareSnapshot(ctx context.Context, id string, parentImage string, platform *ocispec.Platform) error {
	img, err := i.resolveImage(ctx, parentImage)
	if err != nil {
		return err
	}

	cs := i.client.ContentStore()

	matcher := platforms.Default()
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

	parent := identity.ChainID(diffIDs).String()

	// Add a lease so that containerd doesn't garbage collect our snapshot
	ls := i.client.LeasesService()
	lease, err := ls.Create(ctx, leases.WithID(id))
	if err != nil {
		return err
	}

	if err := ls.AddResource(ctx, lease, leases.Resource{
		ID:   id,
		Type: "snapshots/" + i.StorageDriver(),
	}); err != nil {
		return err
	}

	s := i.client.SnapshotService(i.StorageDriver())
	_, err = s.Prepare(ctx, id, parent)
	return err
}
