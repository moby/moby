package containerd

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Children returns a slice of image ID which rootfs is a superset of the
// rootfs of the given image ID, excluding images with exactly the same rootfs.
// Called from list.go to filter containers.
func (i *ImageService) Children(ctx context.Context, id image.ID) ([]image.ID, error) {
	target, err := i.resolveDescriptor(ctx, id.String())
	if err != nil {
		return []image.ID{}, fmt.Errorf("failed to get parent image: %w", err)
	}

	cs := i.client.ContentStore()

	allPlatforms, err := containerdimages.Platforms(ctx, cs, target)
	if err != nil {
		return []image.ID{}, errdefs.System(fmt.Errorf("failed to list platforms supported by image: %w", err))
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
		return []image.ID{}, errdefs.System(fmt.Errorf("failed to list all images: %w", err))
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
		return empty, fmt.Errorf("failed to get config for platform %s: %w", platforms.Format(platform), err)
	}

	diffs, err := containerdimages.RootFS(ctx, store, configDesc)
	if err != nil {
		return empty, fmt.Errorf("failed to obtain rootfs: %w", err)
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
		return nil, fmt.Errorf("failed to get child image: %w", err)
	}

	cs := i.client.ContentStore()

	allPlatforms, err := containerdimages.Platforms(ctx, cs, target)
	if err != nil {
		return nil, errdefs.System(fmt.Errorf("failed to list platforms supported by image: %w", err))
	}

	var childRootFS []ocispec.RootFS
	for _, platform := range allPlatforms {
		rootfs, err := platformRootfs(ctx, cs, target, platform)
		if err != nil {
			if cerrdefs.IsNotFound(err) {
				continue
			}
			return nil, errdefs.System(fmt.Errorf("failed to get platform-specific rootfs: %w", err))
		}

		childRootFS = append(childRootFS, rootfs)
	}

	imgs, err := i.client.ImageService().List(ctx)
	if err != nil {
		return nil, errdefs.System(fmt.Errorf("failed to list all images: %w", err))
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
				return nil, errdefs.System(fmt.Errorf("failed to get platform-specific rootfs: %w", err))
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
