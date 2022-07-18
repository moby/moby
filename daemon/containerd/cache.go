package containerd

import (
	"github.com/docker/docker/builder"
)

// MakeImageCache creates a stateful image cache.
func (i *ImageService) MakeImageCache(cacheFrom []string) builder.ImageCache {
	panic("not implemented")
}
