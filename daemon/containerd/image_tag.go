package containerd

import (
	"context"
	"fmt"

	containerdimages "github.com/containerd/containerd/images"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/pkg/errors"
)

// TagImage creates an image named as newTag and targeting the given descriptor id.
func (i *ImageService) TagImage(ctx context.Context, imageID image.ID, newTag reference.Named) error {
	targetImage, err := i.resolveImage(ctx, imageID.String())
	if err != nil {
		return errors.Wrapf(err, "failed to resolve image id %q to a descriptor", imageID.String())
	}

	newImg := containerdimages.Image{
		Name:   newTag.String(),
		Target: targetImage.Target,
		Labels: targetImage.Labels,
	}

	return i.forceCreateImage(ctx, newImg)
}

// forceCreateImage creates a new image with the given name and target descriptor.
// If an image with the same name already exists, it will be replaced.
// Overwritten image will be persisted as a dangling image if it's a last
// reference to that image.
func (i *ImageService) forceCreateImage(ctx context.Context, newImg containerdimages.Image) error {
	_, err := i.images.Create(ctx, newImg)
	if err != nil {
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
			i.LogImageEvent(newImg.Name, imageFamiliarName(newImg), events.ActionTag)
			return nil
		}

		// If there already exists an image with this tag, delete it
		if err := i.softImageDelete(ctx, *replacedImg, all); err != nil {
			return errors.Wrapf(err, "failed to delete previous image %s", replacedImg.Name)
		}

		if _, err = i.images.Create(context.WithoutCancel(ctx), newImg); err != nil {
			return errdefs.System(errors.Wrapf(err, "failed to create an image %s with target %s after deleting the existing one",
				newImg.Name, newImg.Target.Digest))
		}
	}

	logger := log.G(ctx).WithFields(log.Fields{
		"imageID": newImg.Target,
		"tag":     newImg.Name,
	})
	logger.Info("image created")

	defer i.LogImageEvent(newImg.Name, imageFamiliarName(newImg), events.ActionTag)

	// Delete the source dangling image, as it's no longer dangling.
	if err := i.images.Delete(context.WithoutCancel(ctx), danglingImageName(newImg.Target.Digest)); err != nil {
		logger.WithError(err).Warn("unexpected error when deleting dangling image")
	}

	return nil
}
