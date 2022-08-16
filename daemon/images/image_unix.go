//go:build linux || freebsd
// +build linux freebsd

package images // import "github.com/docker/docker/daemon/images"

import (
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/sirupsen/logrus"
)

// GetLayerFolders returns the layer folders from an image RootFS
func (i *ImageService) GetLayerFolders(img *image.Image, rwLayer layer.RWLayer) ([]string, error) {
	// Windows specific
	panic("not implemented")
}

// GetContainerLayerSize returns the real size & virtual size of the container.
func (i *ImageService) GetContainerLayerSize(containerID string) (int64, int64) {
	var (
		sizeRw, sizeRootfs int64
		err                error
	)

	// Safe to index by runtime.GOOS as Unix hosts don't support multiple
	// container operating systems.
	rwlayer, err := i.layerStore.GetRWLayer(containerID)
	if err != nil {
		logrus.Errorf("Failed to compute size of container rootfs %v: %v", containerID, err)
		return sizeRw, sizeRootfs
	}
	defer i.layerStore.ReleaseRWLayer(rwlayer)

	sizeRw, err = rwlayer.Size()
	if err != nil {
		logrus.Errorf("Driver %s couldn't return diff size of container %s: %s",
			i.layerStore.DriverName(), containerID, err)
		// FIXME: GetSize should return an error. Not changing it now in case
		// there is a side-effect.
		sizeRw = -1
	}

	if parent := rwlayer.Parent(); parent != nil {
		sizeRootfs = parent.Size()
		if sizeRw != -1 {
			sizeRootfs += sizeRw
		}
	}
	return sizeRw, sizeRootfs
}
