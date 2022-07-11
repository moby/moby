package containerd

import (
	"context"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/pkg/errors"
)

// TagImage creates the tag specified by newTag, pointing to the image named
// imageName (alternatively, imageName can also be an image ID).
func (i *ImageService) TagImage(ctx context.Context, imageName, repository, tag string) (string, error) {
	return "", errdefs.NotImplemented(errors.New("not implemented"))
}

// TagImageWithReference adds the given reference to the image ID provided.
func (i *ImageService) TagImageWithReference(ctx context.Context, imageID image.ID, newTag reference.Named) error {
	return errdefs.NotImplemented(errors.New("not implemented"))
}
