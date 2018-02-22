package daemon // import "github.com/docker/docker/daemon"

import (
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"
)

// GetLayerFolders returns the layer folders from an image RootFS
func (daemon *Daemon) GetLayerFolders(img *image.Image, rwLayer layer.RWLayer) ([]string, error) {
	folders := []string{}
	max := len(img.RootFS.DiffIDs)
	for index := 1; index <= max; index++ {
		// FIXME: why does this mutate the RootFS?
		img.RootFS.DiffIDs = img.RootFS.DiffIDs[:index]
		if !system.IsOSSupported(img.OperatingSystem()) {
			return nil, errors.Wrapf(system.ErrNotSupportedOperatingSystem, "cannot get layerpath for ImageID %s", img.RootFS.ChainID())
		}
		layerPath, err := layer.GetLayerPath(daemon.layerStores[img.OperatingSystem()], img.RootFS.ChainID())
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get layer path from graphdriver %s for ImageID %s", daemon.layerStores[img.OperatingSystem()], img.RootFS.ChainID())
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
