package images // import "github.com/docker/docker/daemon/images"

import (
	"context"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/image"
)

// TagImage adds the given reference to the image ID provided.
func (i *ImageService) TagImage(ctx context.Context, imageID image.ID, newTag reference.Named) error {
	if err := i.referenceStore.AddTag(newTag, imageID.Digest(), true); err != nil {
		return err
	}

	if err := i.imageStore.SetLastUpdated(imageID); err != nil {
		return err
	}
	i.LogImageEvent(imageID.String(), reference.FamiliarString(newTag), "tag")
	return nil
}
