package containerd

import (
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/image"
)

// TagImage creates the tag specified by newTag, pointing to the image named
// imageName (alternatively, imageName can also be an image ID).
func (i *ImageService) TagImage(imageName, repository, tag string) (string, error) {
	panic("not implemented")
}

// TagImageWithReference adds the given reference to the image ID provided.
func (i *ImageService) TagImageWithReference(imageID image.ID, newTag reference.Named) error {
	panic("not implemented")
}
