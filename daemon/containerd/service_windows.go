package containerd

import (
	"context"

	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/pkg/errors"
)

// GetLayerFolders returns the layer folders from an image RootFS.
func (i *ImageService) GetLayerFolders(img *image.Image, rwLayer layer.RWLayer, containerID string) ([]string, error) {
	if rwLayer != nil {
		return nil, errors.New("RWLayer is unexpectedly not nil")
	}

	snapshotter := i.client.SnapshotService(i.StorageDriver())
	mounts, err := snapshotter.Mounts(context.TODO(), containerID)
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
