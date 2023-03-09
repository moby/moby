package images

import (
	"context"
)

// GetContainerLayerSize returns real size & virtual size
func (i *ImageService) GetContainerLayerSize(ctx context.Context, containerID string) (int64, int64, error) {
	// TODO Windows
	return 0, 0, nil
}
