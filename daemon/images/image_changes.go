package images

import (
	"context"
	"fmt"

	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/archive"
)

func (i *ImageService) Changes(ctx context.Context, container *container.Container) ([]archive.Change, error) {
	container.Lock()
	defer container.Unlock()

	rwLayer, err := i.layerStore.GetRWLayer(container.ID)
	if err != nil {
		return nil, fmt.Errorf("RWLayer of container "+container.Name+" is unexpectedly nil: %w", err)
	}
	return rwLayer.Changes()
}
