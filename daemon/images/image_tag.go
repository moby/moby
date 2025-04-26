package images // import "github.com/docker/docker/daemon/images"

import (
	"context"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// TagImage adds the given reference to the image ID provided.
func (i *ImageService) TagImage(ctx context.Context, img ocispec.Descriptor, newTag reference.Named) error {
	if err := i.referenceStore.AddTag(newTag, img.Digest, true); err != nil {
		return err
	}

	if err := i.imageStore.SetLastUpdated(image.ID(img.Digest)); err != nil {
		return err
	}
	i.LogImageEvent(ctx, img.Digest.String(), reference.FamiliarString(newTag), events.ActionTag)
	return nil
}
