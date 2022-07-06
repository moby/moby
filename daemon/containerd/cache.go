package containerd

import (
	"context"

	"github.com/docker/docker/builder"
)

// MakeImageCache creates a stateful image cache.
func (i *ImageService) MakeImageCache(ctx context.Context, cacheFrom []string) builder.ImageCache {
	panic("not implemented")
}
