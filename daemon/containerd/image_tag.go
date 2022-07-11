package containerd

import (
	"context"

	containerdimages "github.com/containerd/containerd/images"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/image"
	"github.com/sirupsen/logrus"
)

// TagImage creates the tag specified by newTag, pointing to the image named
// imageName (alternatively, imageName can also be an image ID).
func (i *ImageService) TagImage(ctx context.Context, imageName, repository, tag string) (string, error) {
	desc, err := i.ResolveImage(ctx, imageName)
	if err != nil {
		return "", err
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

	err = i.TagImageWithReference(ctx, image.ID(desc.Digest), newTag)
	return reference.FamiliarString(newTag), err
}

// TagImageWithReference adds the given reference to the image ID provided.
func (i *ImageService) TagImageWithReference(ctx context.Context, imageID image.ID, newTag reference.Named) error {
	logrus.Infof("Tagging image %q with reference %q", imageID, newTag.String())

	desc, err := i.ResolveImage(ctx, imageID.String())
	if err != nil {
		return err
	}

	img := containerdimages.Image{
		Name:   newTag.String(),
		Target: desc,
	}

	is := i.client.ImageService()
	_, err = is.Create(ctx, img)

	return err
}
