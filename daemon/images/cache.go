package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/containerd/log"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/image"
	"github.com/docker/docker/image/cache"
	"github.com/docker/docker/layer"
)

type cacheAdaptor struct {
	is *ImageService
}

func (c cacheAdaptor) Get(id image.ID) (*image.Image, error) {
	return c.is.imageStore.Get(id)
}

func (c cacheAdaptor) GetByRef(ctx context.Context, refOrId string) (*image.Image, error) {
	return c.is.GetImage(ctx, refOrId, backend.GetImageOpts{})
}

func (c cacheAdaptor) SetParent(target, parent image.ID) error {
	return c.is.imageStore.SetParent(target, parent)
}

func (c cacheAdaptor) GetParent(target image.ID) (image.ID, error) {
	return c.is.imageStore.GetParent(target)
}

func (c cacheAdaptor) IsBuiltLocally(target image.ID) (bool, error) {
	return c.is.imageStore.IsBuiltLocally(target)
}

func (c cacheAdaptor) Children(imgID image.ID) []image.ID {
	// Not FROM scratch
	if imgID != "" {
		return c.is.imageStore.Children(imgID)
	}
	images := c.is.imageStore.Map()

	var siblings []image.ID
	for id, img := range images {
		if img.Parent != "" {
			continue
		}

		builtLocally, err := c.is.imageStore.IsBuiltLocally(id)
		if err != nil {
			log.G(context.TODO()).WithFields(log.Fields{
				"error": err,
				"id":    id,
			}).Warn("failed to check if image was built locally")
			continue
		}
		if !builtLocally {
			continue
		}

		siblings = append(siblings, id)
	}
	return siblings
}

func (c cacheAdaptor) Create(parent *image.Image, image image.Image, _ layer.DiffID) (image.ID, error) {
	data, err := json.Marshal(image)
	if err != nil {
		return "", fmt.Errorf("failed to marshal image config: %w", err)
	}
	imgID, err := c.is.imageStore.Create(data)
	if err != nil {
		return "", err
	}

	if parent != nil {
		if err := c.is.imageStore.SetParent(imgID, parent.ID()); err != nil {
			return "", fmt.Errorf("failed to set parent for %v to %v: %w", imgID, parent.ID(), err)
		}
	}

	return imgID, err
}

// MakeImageCache creates a stateful image cache.
func (i *ImageService) MakeImageCache(ctx context.Context, sourceRefs []string) (builder.ImageCache, error) {
	return cache.New(ctx, cacheAdaptor{i}, sourceRefs)
}
