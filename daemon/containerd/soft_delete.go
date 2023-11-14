package containerd

import (
	"context"

	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"

	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/internal/compatcontext"
)

const imageNameDanglingPrefix = "moby-dangling@"

// softImageDelete deletes the image, making sure that there are other images
// that reference the content of the deleted image.
// If no other image exists, a dangling one is created.
func (i *ImageService) softImageDelete(ctx context.Context, img containerdimages.Image) error {
	is := i.client.ImageService()

	// If the image already exists, persist it as dangling image
	// but only if no other image has the same target.
	dgst := img.Target.Digest.String()
	imgs, err := is.List(ctx, "target.digest=="+dgst)
	if err != nil {
		return errdefs.System(errors.Wrapf(err, "failed to check if there are images targeting digest %s", dgst))
	}

	// From this point explicitly ignore the passed context
	// and don't allow to interrupt operation in the middle.

	// Create dangling image if this is the last image pointing to this target.
	if len(imgs) == 1 {
		err = i.ensureDanglingImage(compatcontext.WithoutCancel(ctx), img)

		// Error out in case we couldn't persist the old image.
		if err != nil {
			return errdefs.System(errors.Wrapf(err, "failed to create a dangling image for the replaced image %s with digest %s",
				img.Name, img.Target.Digest.String()))
		}
	}

	// Free the target name.
	err = is.Delete(compatcontext.WithoutCancel(ctx), img.Name)
	if err != nil {
		if !cerrdefs.IsNotFound(err) {
			return errdefs.System(errors.Wrapf(err, "failed to delete image %s which existed a moment before", img.Name))
		}
	}

	return nil
}

func (i *ImageService) ensureDanglingImage(ctx context.Context, from containerdimages.Image) error {
	danglingImage := from

	danglingImage.Labels = make(map[string]string)
	for k, v := range from.Labels {
		switch k {
		case containerdimages.AnnotationImageName, ocispec.AnnotationRefName:
			// Don't copy name labels.
		default:
			danglingImage.Labels[k] = v
		}
	}
	danglingImage.Name = danglingImageName(from.Target.Digest)

	_, err := i.client.ImageService().Create(compatcontext.WithoutCancel(ctx), danglingImage)
	// If it already exists, then just continue.
	if cerrdefs.IsAlreadyExists(err) {
		return nil
	}

	return err
}

func danglingImageName(digest digest.Digest) string {
	return imageNameDanglingPrefix + digest.String()
}

func isDanglingImage(image containerdimages.Image) bool {
	return image.Name == danglingImageName(image.Target.Digest)
}
