package containerd

import (
	"context"

	"github.com/docker/docker/api/types/container"
	imagetype "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/image"
)

// MakeImageCache creates a stateful image cache.
func (i *ImageService) MakeImageCache(ctx context.Context, cacheFrom []string) (builder.ImageCache, error) {
	images := []*image.Image{}
	for _, c := range cacheFrom {
		im, err := i.GetImage(context.TODO(), c, imagetype.GetImageOpts{})
		if err != nil {
			return nil, err
		}
		images = append(images, im)
	}
	return &imageCache{images: images, c: i}, nil
}

type imageCache struct {
	images []*image.Image
	c      *ImageService
}

func (ic *imageCache) GetCache(parentID string, cfg *container.Config) (imageID string, err error) {
	i, err := ic.c.GetImage(context.TODO(), parentID, imagetype.GetImageOpts{})
	if err != nil {
		for _, ii := range ic.images {
			if ii.ID().String() == parentID {
				return ii.ID().String(), nil
			}
		}
	} else {
		return i.ID().String(), nil
	}
	return "", nil
}
