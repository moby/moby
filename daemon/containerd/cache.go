package containerd

import (
	"context"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder"
)

// MakeImageCache creates a stateful image cache.
func (i *ImageService) MakeImageCache(ctx context.Context, cacheFrom []string) (builder.ImageCache, error) {
	return &imageCache{}, nil
}

type imageCache struct{}

func (ic *imageCache) GetCache(parentID string, cfg *container.Config) (imageID string, err error) {
	return "", nil
}
