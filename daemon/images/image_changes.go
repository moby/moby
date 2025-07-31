package images

import (
	"context"
	"errors"
	"fmt"

	"github.com/moby/go-archive"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/internal/layer"
)

func (i *ImageService) Changes(ctx context.Context, container *container.Container) ([]archive.Change, error) {
	container.Lock()
	defer container.Unlock()

	if container.RWLayer == nil {
		return nil, errors.New("RWLayer of container " + container.Name + " is unexpectedly nil")
	}
	rwLayer, ok := container.RWLayer.(layer.RWLayer)
	if !ok {
		return nil, fmt.Errorf("container %s has an unexpected RWLayer type: %T", container.Name, container.RWLayer)
	}
	return rwLayer.Changes()
}
