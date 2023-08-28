package containerd

import (
	"context"
	"fmt"

	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
)

// TagImage creates an image named as newTag and targeting the given descriptor id.
func (i *ImageService) TagImage(ctx context.Context, imageID image.ID, newTag reference.Named) error {
	target, err := i.resolveDescriptor(ctx, imageID.String())
	if err != nil {
		return fmt.Errorf("failed to resolve image id %q to a descriptor: %w", imageID.String(), err)
	}

	newImg := containerdimages.Image{
		Name:   newTag.String(),
		Target: target,
	}

	is := i.client.ImageService()
	_, err = is.Create(ctx, newImg)
	if err != nil {
		if !cerrdefs.IsAlreadyExists(err) {
			return errdefs.System(fmt.Errorf("failed to create image with name %s and target %s: %w", newImg.Name, newImg.Target.Digest.String(), err))
		}

		replacedImg, err := is.Get(ctx, newImg.Name)
		if err != nil {
			return errdefs.Unknown(fmt.Errorf("creating image %s failed because it already exists, but accessing it also failed: %w", newImg.Name, err))
		}

		// Check if image we would replace already resolves to the same target.
		// No need to do anything.
		if replacedImg.Target.Digest == target.Digest {
			i.LogImageEvent(imageID.String(), reference.FamiliarString(newTag), "tag")
			return nil
		}

		// If there already exists an image with this tag, delete it
		if err := i.softImageDelete(ctx, replacedImg); err != nil {
			return fmt.Errorf("failed to delete previous image %s: %w", replacedImg.Name, err)
		}

		if _, err = is.Create(context.Background(), newImg); err != nil {
			return errdefs.System(fmt.Errorf("failed to create an image %s with target %s after deleting the existing one: %w",
				newImg.Name, imageID.String(), err))
		}
	}

	logger := log.G(ctx).WithFields(log.Fields{
		"imageID": imageID.String(),
		"tag":     newTag.String(),
	})
	logger.Info("image created")

	defer i.LogImageEvent(imageID.String(), reference.FamiliarString(newTag), "tag")

	// The tag succeeded, check if the source image is dangling
	sourceDanglingImg, err := is.Get(context.Background(), danglingImageName(target.Digest))
	if err != nil {
		if !cerrdefs.IsNotFound(err) {
			logger.WithError(err).Warn("unexpected error when checking if source image is dangling")
		}

		return nil
	}

	// Delete the source dangling image, as it's no longer dangling.
	if err := is.Delete(context.Background(), sourceDanglingImg.Name); err != nil {
		logger.WithError(err).Warn("unexpected error when deleting dangling image")
	}

	return nil
}
