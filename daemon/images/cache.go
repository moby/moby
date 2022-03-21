package images // import "github.com/docker/docker/daemon/images"

import (
	"context"

	"github.com/docker/docker/builder"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image/cache"
	"github.com/sirupsen/logrus"
)

// MakeImageCache creates a stateful image cache.
func (i *ImageService) MakeImageCache(ctx context.Context, sourceRefs []string) (builder.ImageCache, error) {
	if len(sourceRefs) == 0 {
		return cache.NewLocal(i.imageStore), nil
	}

	cache := cache.New(i.imageStore)

	for _, ref := range sourceRefs {
		img, err := i.GetImage(ctx, ref, nil)
		if err != nil {
			if errdefs.IsNotFound(err) {
				logrus.Warnf("Could not look up %s for cache resolution, skipping: %+v", ref, err)
				continue
			}
			return nil, err
		}
		cache.Populate(img)
	}

	return cache, nil
}
