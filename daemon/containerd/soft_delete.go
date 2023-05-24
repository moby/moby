package containerd

import (
	"context"

	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/docker/docker/errdefs"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// softImageDelete deletes the image, making sure that there are other images
// that reference the content of the deleted image.
// If no other image exists, a dangling one is created.
func (i *ImageService) softImageDelete(ctx context.Context, img containerdimages.Image) error {
	is := i.client.ImageService()

	// If the image already exists, persist it as dangling image
	// but only if no other image has the same target.
	digest := img.Target.Digest.String()
	imgs, err := is.List(ctx, "target.digest=="+digest)
	if err != nil {
		return errdefs.System(errors.Wrapf(err, "failed to check if there are images targeting digest %s", digest))
	}

	// From this point explicitly ignore the passed context
	// and don't allow to interrupt operation in the middle.

	// Create dangling image if this is the last image pointing to this target.
	if len(imgs) == 1 {
		danglingImage := img

		danglingImage.Name = danglingImageName(img.Target.Digest)
		delete(danglingImage.Labels, containerdimages.AnnotationImageName)
		delete(danglingImage.Labels, ocispec.AnnotationRefName)

		_, err = is.Create(context.Background(), danglingImage)

		// Error out in case we couldn't persist the old image.
		// If it already exists, then just continue.
		if err != nil && !cerrdefs.IsAlreadyExists(err) {
			return errdefs.System(errors.Wrapf(err, "failed to create a dangling image for the replaced image %s with digest %s",
				danglingImage.Name, danglingImage.Target.Digest.String()))
		}
	}

	// Free the target name.
	err = is.Delete(context.Background(), img.Name)
	if err != nil {
		if !cerrdefs.IsNotFound(err) {
			return errdefs.System(errors.Wrapf(err, "failed to delete image %s which existed a moment before", img.Name))
		}
	}

	return nil
}

func danglingImageName(digest digest.Digest) string {
	return "moby-dangling@" + digest.String()
}

func isDanglingImage(image containerdimages.Image) bool {
	return image.Name == danglingImageName(image.Target.Digest)
}
