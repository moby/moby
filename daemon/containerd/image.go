package containerd

import (
	imagetype "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/image"
)

// GetImage returns an image corresponding to the image referred to by refOrID.
func (i *ImageService) GetImage(refOrID string, options imagetype.GetImageOpts) (retImg *image.Image, retErr error) {
	panic("not implemented")
}
