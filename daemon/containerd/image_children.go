package containerd

import (
	"context"

	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/docker/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

// Children returns a slice of image ID which rootfs is a superset of the
// rootfs of the given image ID, excluding images with exactly the same rootfs.
// Called from list.go to filter containers.
func (i *ImageService) Children(ctx context.Context, id image.ID) []image.ID {
	target, err := i.resolveDescriptor(ctx, id.String())
	if err != nil {
		logrus.WithError(err).Error("failed to get parent image")
		return []image.ID{}
	}

	is := i.client.ImageService()
	cs := i.client.ContentStore()

	log := logrus.WithField("id", id)

	allPlatforms, err := containerdimages.Platforms(ctx, cs, target)
	if err != nil {
		log.WithError(err).Error("failed to list supported platorms of image")
		return []image.ID{}
	}

	parentRootFS := []ocispec.RootFS{}
	for _, platform := range allPlatforms {
		rootfs, err := platformRootfs(ctx, cs, target, platform)
		if err != nil {
			continue
		}

		parentRootFS = append(parentRootFS, rootfs)
	}

	imgs, err := is.List(ctx)
	if err != nil {
		log.WithError(err).Error("failed to list all images")
		return []image.ID{}
	}

	children := []image.ID{}
	for _, img := range imgs {
	nextImage:
		for _, platform := range allPlatforms {
			rootfs, err := platformRootfs(ctx, cs, img.Target, platform)
			if err != nil {
				continue
			}

			for _, parentRoot := range parentRootFS {
				if isRootfsChildOf(rootfs, parentRoot) {
					children = append(children, image.ID(img.Target.Digest))
					break nextImage
				}
			}
		}

	}

	return children
}

// platformRootfs returns a rootfs for a specified platform.
func platformRootfs(ctx context.Context, store content.Store, desc ocispec.Descriptor, platform ocispec.Platform) (ocispec.RootFS, error) {
	empty := ocispec.RootFS{}

	log := logrus.WithField("desc", desc.Digest).WithField("platform", platforms.Format(platform))
	configDesc, err := containerdimages.Config(ctx, store, desc, platforms.OnlyStrict(platform))
	if err != nil {
		if !cerrdefs.IsNotFound(err) {
			log.WithError(err).Warning("failed to get parent image config")
		}
		return empty, err
	}

	log = log.WithField("configDesc", configDesc)
	diffs, err := containerdimages.RootFS(ctx, store, configDesc)
	if err != nil {
		if !cerrdefs.IsNotFound(err) {
			log.WithError(err).Warning("failed to get parent image rootfs")
		}
		return empty, err
	}

	return ocispec.RootFS{
		Type:    "layers",
		DiffIDs: diffs,
	}, nil
}

// isRootfsChildOf checks if all layers from parent rootfs are child's first layers
// and child has at least one more layer (to make it not commutative).
// Example:
// A with layers [X, Y],
// B with layers [X, Y, Z]
// C with layers [Y, Z]
//
// Only isRootfsChildOf(B, A) is true.
// Which means that B is considered a children of A. B and C has no children.
// See more examples in TestIsRootfsChildOf.
func isRootfsChildOf(child ocispec.RootFS, parent ocispec.RootFS) bool {
	childLen := len(child.DiffIDs)
	parentLen := len(parent.DiffIDs)

	if childLen <= parentLen {
		return false
	}

	for i := 0; i < parentLen; i++ {
		if child.DiffIDs[i] != parent.DiffIDs[i] {
			return false
		}
	}

	return true
}
