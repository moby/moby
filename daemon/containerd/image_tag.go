package containerd

import (
	"errors"

	reference "github.com/containerd/containerd/reference/docker"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
)

// TagImage creates the tag specified by newTag, pointing to the image named
// imageName (alternatively, imageName can also be an image ID).
func (i *ImageService) TagImage(imageName, repository, tag string) (string, error) {
	return "", errdefs.NotImplemented(errors.New("not implemented"))
}

// TagImageWithReference adds the given reference to the image ID provided.
func (i *ImageService) TagImageWithReference(imageID image.ID, newTag reference.Named) error {
	return errdefs.NotImplemented(errors.New("not implemented"))
}
