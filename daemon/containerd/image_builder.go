package containerd

import (
	"context"

	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/builder"
)

// GetImageAndReleasableLayer returns an image and releaseable layer for a
// reference or ID. Every call to GetImageAndReleasableLayer MUST call
// releasableLayer.Release() to prevent leaking of layers.
func (i *ImageService) GetImageAndReleasableLayer(ctx context.Context, refOrID string, opts backend.GetImageAndLayerOptions) (builder.Image, builder.ROLayer, error) {
	panic("not implemented")
}

// CreateImage creates a new image by adding a config and ID to the image store.
// This is similar to LoadImage() except that it receives JSON encoded bytes of
// an image instead of a tar archive.
func (i *ImageService) CreateImage(config []byte, parent string) (builder.Image, error) {
	panic("not implemented")
}
