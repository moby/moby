package containerd

import (
	"context"
	"encoding/json"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/archive"
	"github.com/google/uuid"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func (i *ImageService) Changes(ctx context.Context, container *container.Container) ([]archive.Change, error) {
	cs := i.client.ContentStore()

	imageManifest, err := getContainerImageManifest(container)
	if err != nil {
		return nil, err
	}

	imageManifestBytes, err := content.ReadBlob(ctx, cs, imageManifest)
	if err != nil {
		return nil, err
	}
	var manifest ocispec.Manifest
	if err := json.Unmarshal(imageManifestBytes, &manifest); err != nil {
		return nil, err
	}

	imageConfigBytes, err := content.ReadBlob(ctx, cs, manifest.Config)
	if err != nil {
		return nil, err
	}
	var image ocispec.Image
	if err := json.Unmarshal(imageConfigBytes, &image); err != nil {
		return nil, err
	}

	rnd, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	snapshotter := i.client.SnapshotService(container.Driver)

	diffIDs := image.RootFS.DiffIDs
	parent, err := snapshotter.View(ctx, rnd.String(), identity.ChainID(diffIDs).String())
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := snapshotter.Remove(ctx, rnd.String()); err != nil {
			log.G(ctx).WithError(err).WithField("key", rnd.String()).Warn("remove temporary snapshot")
		}
	}()

	mounts, err := snapshotter.Mounts(ctx, container.ID)
	if err != nil {
		return nil, err
	}

	var changes []archive.Change
	err = mount.WithReadonlyTempMount(ctx, mounts, func(fs string) error {
		return mount.WithTempMount(ctx, parent, func(root string) error {
			changes, err = archive.ChangesDirs(fs, root)
			return err
		})
	})
	return changes, err
}
