package containerd

import (
	"context"

	c8dimages "github.com/containerd/containerd/v2/core/images"
	"github.com/moby/moby/v2/daemon/internal/image"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

// getImagesWithLabel returns all images that have the matching label key and value.
func (i *ImageService) getImagesWithLabel(ctx context.Context, labelKey string, labelValue string) ([]image.ID, error) {
	imgs, err := i.images.List(ctx, "labels."+labelKey+"=="+labelValue)
	if err != nil {
		return []image.ID{}, types.SystemErrorf("failed to list all images: %w", err)
	}

	var children []image.ID
	for _, img := range imgs {
		children = append(children, image.ID(img.Target.Digest))
	}

	return children, nil
}

// Children returns a slice of image IDs that are children of the `id` image
func (i *ImageService) Children(ctx context.Context, id image.ID) ([]image.ID, error) {
	return i.getImagesWithLabel(ctx, imageLabelClassicBuilderParent, string(id))
}

// parents returns a slice of image IDs that are parents of the `id` image
//
// Called from image_delete.go to prune dangling parents.
func (i *ImageService) parents(ctx context.Context, id image.ID) ([]c8dimages.Image, error) {
	targetImage, err := i.resolveImage(ctx, id.String())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get child image")
	}

	var imgs []c8dimages.Image
	for {
		parent, ok := targetImage.Labels[imageLabelClassicBuilderParent]
		if !ok || parent == "" {
			break
		}

		parentDigest, err := digest.Parse(parent)
		if err != nil {
			return nil, err
		}
		img, err := i.resolveImage(ctx, parentDigest.String())
		if err != nil {
			return nil, err
		}
		imgs = append(imgs, img)
		targetImage = img
	}

	return imgs, nil
}
