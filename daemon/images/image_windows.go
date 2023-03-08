package images

import (
	"context"

	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"
)

// GetContainerLayerSize returns real size & virtual size
func (i *ImageService) GetContainerLayerSize(ctx context.Context, containerID string) (int64, int64, error) {
	// TODO Windows
	return 0, 0, nil
}

// GetLayerFolders returns the layer folders from an image RootFS
func (i *ImageService) GetLayerFolders(img *image.Image, rwLayer layer.RWLayer) ([]string, error) {
	folders := []string{}
	max := len(img.RootFS.DiffIDs)
	for index := 1; index <= max; index++ {
		// FIXME: why does this mutate the RootFS?
		img.RootFS.DiffIDs = img.RootFS.DiffIDs[:index]
		if !system.IsOSSupported(img.OperatingSystem()) {
			return nil, errors.Wrapf(system.ErrNotSupportedOperatingSystem, "cannot get layerpath for ImageID %s", img.RootFS.ChainID())
		}
		layerPath, err := layer.GetLayerPath(i.layerStore, img.RootFS.ChainID())
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get layer path from graphdriver %s for ImageID %s", i.layerStore, img.RootFS.ChainID())
		}
		// Reverse order, expecting parent first
		folders = append([]string{layerPath}, folders...)
	}
	if rwLayer == nil {
		return nil, errors.New("RWLayer is unexpectedly nil")
	}
	m, err := rwLayer.Metadata()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get layer metadata")
	}
	return append(folders, m["dir"]), nil
}
