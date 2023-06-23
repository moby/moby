package images // import "github.com/docker/docker/daemon/images"

import (
	"context"

	"github.com/containerd/containerd/log"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/image/cache"
	"github.com/pkg/errors"
)

// MakeImageCache creates a stateful image cache.
func (i *ImageService) MakeImageCache(ctx context.Context, sourceRefs []string) (builder.ImageCache, error) {
	if len(sourceRefs) == 0 {
		return cache.NewLocal(i.imageStore), nil
	}

	cache := cache.New(i.imageStore)

	for _, ref := range sourceRefs {
		img, err := i.GetImage(ctx, ref, imagetypes.GetImageOpts{})
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			log.G(ctx).Warnf("Could not look up %s for cache resolution, skipping: %+v", ref, err)
			continue
		}
		cache.Populate(img)
	}

	return cache, nil
}
