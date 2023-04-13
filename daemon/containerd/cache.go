package containerd

import (
	"context"
	"reflect"

	"github.com/docker/docker/api/types/container"
	imagetype "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/image"
)

// MakeImageCache creates a stateful image cache.
func (i *ImageService) MakeImageCache(ctx context.Context, cacheFrom []string) (builder.ImageCache, error) {
	images := []*image.Image{}
	for _, c := range cacheFrom {
		im, err := i.GetImage(ctx, c, imagetype.GetImageOpts{})
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
	ctx := context.TODO()
	cfgCpy := *cfg
	i, err := ic.c.GetImage(ctx, parentID, imagetype.GetImageOpts{})
	if err != nil {
		for _, ii := range ic.images {
			if ii.ID().String() == parentID {
				if compare(ii.RunConfig(), &cfgCpy) {
					return ii.ID().String(), nil
				}
			}
		}
	} else {
		children, err := ic.c.Children(ctx, i.ID())
		if err != nil {
			return "", err
		}
		for _, ch := range children {
			childImage, err := ic.c.GetImage(context.TODO(), ch.String(), imagetype.GetImageOpts{})
			if err != nil {
				return "", err
			}
			// this implementation looks correct but it's currently not working
			// with the containerd store as we're not storing the image ContainerConfig
			// and so intermediate images with ContainerConfigs such as
			// #(nop) COPY file:c6ab44934e83eeb07289a211582c6faa25dea7d06dae077b6ef76029e92400ce in ...
			// are not getting a hit
			if compare(&childImage.ContainerConfig, &cfgCpy) {
				return ch.String(), nil
			}
		}
	}
	return "", nil
}

// compare two Config structs. Do not consider the "Hostname" field as it
// defaults to the randomly generated short container ID or "Image" as it
// represents the name of the image as it was passed in (symbolic) and does
// not provide any meaningful information about whether the image is usable
// as cache.
// If OpenStdin is set, then it differs
func compare(a, b *container.Config) bool {
	if a == nil || b == nil ||
		a.OpenStdin || b.OpenStdin {
		return false
	}

	a.Image = ""
	a.Hostname = ""
	b.Image = ""
	b.Hostname = ""
	return reflect.DeepEqual(a, b)
}
