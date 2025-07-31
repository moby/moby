package containerd

import (
	"context"
	"fmt"

	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/internal/image"
	"github.com/pkg/errors"
)

// GetLayerFolders returns the layer folders from an image RootFS.
func (i *ImageService) GetLayerFolders(img *image.Image, layer container.RWLayer, containerID string) ([]string, error) {
	if layer == nil {
		return nil, errors.New("RWLayer is unexpectedly nil")
	}

	c8dLayer, ok := layer.(*rwLayer)
	if !ok {
		return nil, fmt.Errorf("unexpected layer type: %T", layer)
	}

	mounts, err := c8dLayer.mounts(context.TODO())
	if err != nil {
		return nil, errors.Wrapf(err, "snapshotter.Mounts failed: container %s", containerID)
	}

	// This is the same logic used by the hcsshim containerd runtime shim's createInternal
	// to convert an array of Mounts into windows layers.
	// See https://github.com/microsoft/hcsshim/blob/release/0.11/cmd/containerd-shim-runhcs-v1/service_internal.go
	parentPaths, err := mounts[0].GetParentPaths()
	if err != nil {
		return nil, errors.Wrapf(err, "GetParentPaths failed: container %s", containerID)
	}
	return append(parentPaths, mounts[0].Source), nil
}
