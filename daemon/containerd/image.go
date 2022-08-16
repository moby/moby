package containerd

import (
	"github.com/docker/docker/image"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// GetImage returns an image corresponding to the image referred to by refOrID.
func (i *ImageService) GetImage(refOrID string, platform *specs.Platform) (retImg *image.Image, retErr error) {
	panic("not implemented")
}
