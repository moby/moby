package containerd

import (
	"context"

	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// Children returns a slice of image ID which rootfs is a superset of the
// rootfs of the given image ID, excluding images with exactly the same rootfs.
// Called from list.go to filter containers.
func (i *ImageService) Children(ctx context.Context, id image.ID) ([]image.ID, error) {
	target, err := i.resolveDescriptor(ctx, id.String())
	if err != nil {
		return []image.ID{}, errors.Wrap(err, "failed to get parent image")
	}

	cs := i.client.ContentStore()

	allPlatforms, err := containerdimages.Platforms(ctx, cs, target)
	if err != nil {
		return []image.ID{}, errdefs.System(errors.Wrap(err, "failed to list platforms supported by image"))
	}

	parentRootFS := []ocispec.RootFS{}
	for _, platform := range allPlatforms {
		rootfs, err := platformRootfs(ctx, cs, target, platform)
		if err != nil {
			if !cerrdefs.IsNotFound(err) {
				log.G(ctx).WithFields(log.Fields{
					"error":    err,
					"image":    target.Digest,
					"platform": platform,
				}).Warning("failed to get platform-specific rootfs")
			}
			continue
		}

		parentRootFS = append(parentRootFS, rootfs)
	}

	imgs, err := i.client.ImageService().List(ctx)
	if err != nil {
		return []image.ID{}, errdefs.System(errors.Wrap(err, "failed to list all images"))
	}

	children := []image.ID{}
	for _, img := range imgs {
	nextImage:
		for _, platform := range allPlatforms {
			rootfs, err := platformRootfs(ctx, cs, img.Target, platform)
			if err != nil {
				if !cerrdefs.IsNotFound(err) {
					log.G(ctx).WithFields(log.Fields{
						"error":    err,
						"image":    img.Target.Digest,
						"platform": platform,
					}).Warning("failed to get platform-specific rootfs")
				}
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

	return children, nil
}

// platformRootfs returns a rootfs for a specified platform.
func platformRootfs(ctx context.Context, store content.Store, desc ocispec.Descriptor, platform ocispec.Platform) (ocispec.RootFS, error) {
	empty := ocispec.RootFS{}

	configDesc, err := containerdimages.Config(ctx, store, desc, platforms.OnlyStrict(platform))
	if err != nil {
		return empty, errors.Wrapf(err, "failed to get config for platform %s", platforms.Format(platform))
	}

	diffs, err := containerdimages.RootFS(ctx, store, configDesc)
	if err != nil {
		return empty, errors.Wrapf(err, "failed to obtain rootfs")
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

// parents returns a slice of image IDs whose entire rootfs contents match,
// in order, the childs first layers, excluding images with the exact same
// rootfs.
//
// Called from image_delete.go to prune dangling parents.
func (i *ImageService) parents(ctx context.Context, id image.ID) ([]imageWithRootfs, error) {
	target, err := i.resolveDescriptor(ctx, id.String())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get child image")
	}

	cs := i.client.ContentStore()

	allPlatforms, err := containerdimages.Platforms(ctx, cs, target)
	if err != nil {
		return nil, errdefs.System(errors.Wrap(err, "failed to list platforms supported by image"))
	}

	var childRootFS []ocispec.RootFS
	for _, platform := range allPlatforms {
		rootfs, err := platformRootfs(ctx, cs, target, platform)
		if err != nil {
			if cerrdefs.IsNotFound(err) {
				continue
			}
			return nil, errdefs.System(errors.Wrap(err, "failed to get platform-specific rootfs"))
		}

		childRootFS = append(childRootFS, rootfs)
	}

	imgs, err := i.client.ImageService().List(ctx)
	if err != nil {
		return nil, errdefs.System(errors.Wrap(err, "failed to list all images"))
	}

	var parents []imageWithRootfs
	for _, img := range imgs {
	nextImage:
		for _, platform := range allPlatforms {
			rootfs, err := platformRootfs(ctx, cs, img.Target, platform)
			if err != nil {
				if cerrdefs.IsNotFound(err) {
					continue
				}
				return nil, errdefs.System(errors.Wrap(err, "failed to get platform-specific rootfs"))
			}

			for _, childRoot := range childRootFS {
				if isRootfsChildOf(childRoot, rootfs) {
					parents = append(parents, imageWithRootfs{
						img:    img,
						rootfs: rootfs,
					})
					break nextImage
				}
			}
		}
	}

	return parents, nil
}

type imageWithRootfs struct {
	img    containerdimages.Image
	rootfs ocispec.RootFS
}
