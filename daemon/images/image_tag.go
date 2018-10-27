package images // import "github.com/docker/docker/daemon/images"

import (
	"context"

	"github.com/containerd/containerd/images"
	"github.com/docker/distribution/reference"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// TagImage creates the tag specified by newTag, pointing to the image named
// imageName (alternatively, imageName can also be an image ID).
func (i *ImageService) TagImage(imageName, repository, tag string) (string, error) {
	// TODO(containerd): Lookup existing image descriptor
	img, err := i.GetImage(imageName)
	if err != nil {
		return "", err
	}

	var target ocispec.Descriptor
	if len(img.References) > 0 {
		target = img.References[0]
	} else {
		target = img.Config
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

	err = i.TagImageWithReference(target, newTag)
	return reference.FamiliarString(newTag), err
}

// TagImageWithReference adds the given reference to the image ID provided.
func (i *ImageService) TagImageWithReference(target ocispec.Descriptor, newTag reference.Named) error {
	img := images.Image{
		Name:   newTag.String(),
		Target: target,
	}
	is := i.client.ImageService()
	_, err := is.Create(context.TODO(), img)
	if err != nil {
		return errors.Wrap(err, "failed to create image")
	}
	// TODO(containerd): Set last updated for target
	i.LogImageEvent(target.Digest.String(), reference.FamiliarString(newTag), "tag")
	return nil
}
