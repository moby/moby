package containerd

import (
	"context"

	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/internal/compatcontext"
	"github.com/pkg/errors"
)

// TagImage creates an image named as newTag and targeting the given descriptor id.
func (i *ImageService) TagImage(ctx context.Context, imageID image.ID, newTag reference.Named) error {
	target, err := i.resolveDescriptor(ctx, imageID.String())
	if err != nil {
		return errors.Wrapf(err, "failed to resolve image id %q to a descriptor", imageID.String())
	}

	newImg := containerdimages.Image{
		Name:   newTag.String(),
		Target: target,
	}

	is := i.client.ImageService()
	_, err = is.Create(ctx, newImg)
	if err != nil {
		if !cerrdefs.IsAlreadyExists(err) {
			return errdefs.System(errors.Wrapf(err, "failed to create image with name %s and target %s", newImg.Name, newImg.Target.Digest.String()))
		}

		replacedImg, err := is.Get(ctx, newImg.Name)
		if err != nil {
			return errdefs.Unknown(errors.Wrapf(err, "creating image %s failed because it already exists, but accessing it also failed", newImg.Name))
		}

		// Check if image we would replace already resolves to the same target.
		// No need to do anything.
		if replacedImg.Target.Digest == target.Digest {
			i.LogImageEvent(imageID.String(), reference.FamiliarString(newTag), events.ActionTag)
			return nil
		}

		// If there already exists an image with this tag, delete it
		if err := i.softImageDelete(ctx, replacedImg); err != nil {
			return errors.Wrapf(err, "failed to delete previous image %s", replacedImg.Name)
		}

		if _, err = is.Create(compatcontext.WithoutCancel(ctx), newImg); err != nil {
			return errdefs.System(errors.Wrapf(err, "failed to create an image %s with target %s after deleting the existing one",
				newImg.Name, imageID.String()))
		}
	}

	logger := log.G(ctx).WithFields(log.Fields{
		"imageID": imageID.String(),
		"tag":     newTag.String(),
	})
	logger.Info("image created")

	defer i.LogImageEvent(imageID.String(), reference.FamiliarString(newTag), events.ActionTag)

	// The tag succeeded, check if the source image is dangling
	sourceDanglingImg, err := is.Get(compatcontext.WithoutCancel(ctx), danglingImageName(target.Digest))
	if err != nil {
		if !cerrdefs.IsNotFound(err) {
			logger.WithError(err).Warn("unexpected error when checking if source image is dangling")
		}

		return nil
	}

	builderLabel, ok := sourceDanglingImg.Labels[imageLabelClassicBuilderParent]
	if ok {
		newImg.Labels = map[string]string{
			imageLabelClassicBuilderParent: builderLabel,
		}

		if _, err := is.Update(compatcontext.WithoutCancel(ctx), newImg, "labels"); err != nil {
			logger.WithError(err).Warnf("failed to set %s label on the newly tagged image", imageLabelClassicBuilderParent)
		}
	}

	// Delete the source dangling image, as it's no longer dangling.
	if err := is.Delete(compatcontext.WithoutCancel(ctx), sourceDanglingImg.Name); err != nil {
		logger.WithError(err).Warn("unexpected error when deleting dangling image")
	}

	return nil
}
