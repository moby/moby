package containerd

import (
	"context"

	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/platforms"
	"github.com/opencontainers/image-spec/identity"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// PrepareSnapshot prepares a snapshot from a parent image for a container
func (i *ImageService) PrepareSnapshot(ctx context.Context, id string, parentImage string, platform *v1.Platform) error {
	desc, err := i.resolveDescriptor(ctx, parentImage)
	if err != nil {
		return err
	}

	cs := i.client.ContentStore()

	matcher := platforms.Default()
	if platform != nil {
		matcher = platforms.Only(*platform)
	}

	desc, err = containerdimages.Config(ctx, cs, desc, matcher)
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
	if _, err := s.Prepare(ctx, id, parent); err == nil {
		return err
	}

	return nil
}
