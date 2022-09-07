package containerd

import (
	"context"

	imagetype "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/image"
)

// GetImage returns an image corresponding to the image referred to by refOrID.
func (i *ImageService) GetImage(ctx context.Context, refOrID string, options imagetype.GetImageOpts) (retImg *image.Image, retErr error) {
	panic("not implemented")
}
