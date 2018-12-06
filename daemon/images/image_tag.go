package images // import "github.com/docker/docker/daemon/images"

import (
	"context"

	"github.com/containerd/containerd/images"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// TagImage creates the tag specified by newTag, pointing to the image named
// imageName (alternatively, imageName can also be an image ID).
func (i *ImageService) TagImage(ctx context.Context, imageName, repository, tag string) (string, error) {
	img, err := i.getImageByRef(ctx, imageName)
	if err != nil {
		return "", err
	}

	if img.target == nil {
		// TODO(containerd): Choose a better target based on other references?
		img.target = &img.cached.config
	}

	newTag, err := reference.ParseNormalizedNamed(repository)
	if err != nil {
		return "", err
	}
	if tag != "" {
		if newTag, err = reference.WithTag(reference.TrimNamed(newTag), tag); err != nil {
			return "", err
		}
	}
	img.name = newTag

	err = i.tagImage(ctx, img)
	return reference.FamiliarString(newTag), err
}

// TagImageWithReference adds the given reference to the image ID provided.
func (i *ImageService) TagImageWithReference(ctx context.Context, target ocispec.Descriptor, newTag reference.Named) error {
	c, err := i.getCache(ctx)
	if err != nil {
		return err
	}
	ci := c.byTarget(target.Digest)
	if ci == nil {
		return errdefs.NotFound(errors.New("target not found"))
	}

	return i.tagImage(ctx, imageLink{
		name:   newTag,
		target: &target,
		cached: ci,
	})
}

func (i *ImageService) tagImage(ctx context.Context, img imageLink) error {
	im := images.Image{
		Name:   img.name.String(),
		Target: *img.target,
	}
	is := i.client.ImageService()
	_, err := is.Create(ctx, im)
	if err != nil {
		return errors.Wrap(err, "failed to create image")
	}

	// TODO(containerd): Set last updated for target
	i.LogImageEvent(img.target.Digest.String(), reference.FamiliarString(img.name), "tag")
	return i.updateCache(ctx, img)
}
