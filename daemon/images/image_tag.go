package images // import "github.com/docker/docker/daemon/images"

import (
	"context"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/image"
)

// TagImage creates the tag specified by newTag, pointing to the image named
// imageName (alternatively, imageName can also be an image ID).
func (i *ImageService) TagImage(ctx context.Context, imageName, repository, tag string) (string, error) {
	// FIXME(thaJeztah): should this use the existing context?
	img, err := i.GetImage(context.TODO(), imageName, nil)
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

	// FIXME(thaJeztah): should this use the existing context?
	err = i.TagImageWithReference(context.TODO(), img.ID(), newTag)
	return reference.FamiliarString(newTag), err
}

// TagImageWithReference adds the given reference to the image ID provided.
func (i *ImageService) TagImageWithReference(ctx context.Context, imageID image.ID, newTag reference.Named) error {
	if err := i.referenceStore.AddTag(newTag, imageID.Digest(), true); err != nil {
		return err
	}

	if err := i.imageStore.SetLastUpdated(imageID); err != nil {
		return err
	}
	i.LogImageEvent(imageID.String(), reference.FamiliarString(newTag), "tag")
	return nil
}
