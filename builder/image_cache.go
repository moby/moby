package builder

import (
	"fmt"

	"github.com/docker/docker/image"
	"github.com/docker/docker/runconfig"
)

type imageCache struct {
	images   map[string]*image.Image
	children map[string]map[string]struct{} // map[parentID][childID]
}

func newImageCache(images map[string]*image.Image) *imageCache {
	children := make(map[string]map[string]struct{})
	for _, img := range images {
		if _, exists := children[img.Parent]; !exists {
			children[img.Parent] = make(map[string]struct{})
		}
		children[img.Parent][img.ID] = struct{}{}
	}

	return &imageCache{
		images:   images,
		children: children,
	}
}

func (cache *imageCache) Dispose() {
	cache.images = nil
	cache.children = nil
}

func (cache *imageCache) Get(parentID string, config *runconfig.Config) (*image.Image, error) {
	// Loop on the children of the given image and check the config
	var match *image.Image
	for childID := range cache.children[parentID] {
		child, exists := cache.images[childID]
		if !exists {
			return nil, fmt.Errorf("no such id: %s", childID)
		}
		if runconfig.Compare(&child.ContainerConfig, config) {
			if match == nil || match.Created.Before(child.Created) {
				match = child
			}
		}
	}
	return match, nil
}
