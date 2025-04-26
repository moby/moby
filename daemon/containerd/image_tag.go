package containerd

import (
	"context"
	"fmt"

	c8dimages "github.com/containerd/containerd/v2/core/images"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// TagImage creates an image named as newTag and targeting the given descriptor id.
func (i *ImageService) TagImage(ctx context.Context, img ocispec.Descriptor, newTag reference.Named) error {
	// TODO: Only lookup if media type is empty
	imgs, err := i.images.List(ctx, "target.digest=="+img.Digest.String())
	if err != nil {
		return fmt.Errorf("failed to lookup digest: %w", err)
	}
	if len(imgs) == 0 {
		return fmt.Errorf("no image found for digest %s: %w", img.Digest.String(), cerrdefs.ErrNotFound)
	}

	newImg := c8dimages.Image{
		Name:   newTag.String(),
		Target: imgs[0].Target,
		// TODO: Consider using annotations from img, could pick up labels
		// not intended for this tag such as GC labels.
		Labels: imgs[0].Labels,
	}

	return i.createOrReplaceImage(ctx, newImg)
}

// createOrReplaceImage creates a new image with the given name and target descriptor.
// If an image with the same name already exists, it will be replaced.
// Overwritten image will be persisted as a dangling image if it's a last
// reference to that image.
func (i *ImageService) createOrReplaceImage(ctx context.Context, newImg c8dimages.Image) error {
	// Delete the source dangling image, as it's no longer dangling.
	// Unless, the image to be created itself is dangling.
	danglingName := danglingImageName(newImg.Target.Digest)

	// The created image is a dangling image.
	creatingDangling := newImg.Name == danglingName

	if _, err := i.images.Create(ctx, newImg); err != nil {
		if !cerrdefs.IsAlreadyExists(err) {
			return errdefs.System(errors.Wrapf(err, "failed to create image with name %s and target %s", newImg.Name, newImg.Target.Digest.String()))
		}

		replacedImg, all, err := i.resolveAllReferences(ctx, newImg.Name)
		if err != nil {
			return errdefs.Unknown(errors.Wrapf(err, "creating image %s failed because it already exists, but accessing it also failed", newImg.Name))
		} else if replacedImg == nil {
			return errdefs.Unknown(fmt.Errorf("creating image %s failed because it already exists, but failed to resolve", newImg.Name))
		}

		// Check if image we would replace already resolves to the same target.
		// No need to do anything.
		if replacedImg.Target.Digest == newImg.Target.Digest {
			if !creatingDangling {
				i.LogImageEvent(ctx, replacedImg.Target.Digest.String(), imageFamiliarName(newImg), events.ActionTag)
			}
			return nil
		}

		// If there already exists an image with this tag, delete it
		if err := i.softImageDelete(ctx, *replacedImg, all); err != nil {
			return errors.Wrapf(err, "failed to delete previous image %s", replacedImg.Name)
		}

		if _, err := i.images.Create(context.WithoutCancel(ctx), newImg); err != nil {
			return errdefs.System(errors.Wrapf(err, "failed to create an image %s with target %s after deleting the existing one",
				newImg.Name, newImg.Target.Digest))
		}
	}

	logger := log.G(ctx).WithFields(log.Fields{
		"imageID": newImg.Target.Digest,
		"tag":     newImg.Name,
	})
	logger.Info("image created")

	if !creatingDangling {
		defer i.LogImageEvent(ctx, string(newImg.Target.Digest), imageFamiliarName(newImg), events.ActionTag)

		if err := i.images.Delete(ctx, danglingName); err != nil {
			if !cerrdefs.IsNotFound(err) {
				logger.WithError(err).Warn("unexpected error when deleting dangling image")
			}
		}
	}

	return nil
}
