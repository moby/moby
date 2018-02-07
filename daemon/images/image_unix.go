// +build linux freebsd

package images // import "github.com/docker/docker/daemon/images"

import (
	"runtime"

	"github.com/sirupsen/logrus"
)

// GetContainerLayerSize returns the real size & virtual size of the container.
func (i *ImageService) GetContainerLayerSize(containerID string) (int64, int64) {
	var (
		sizeRw, sizeRootfs int64
		err                error
	)

	// Safe to index by runtime.GOOS as Unix hosts don't support multiple
	// container operating systems.
	rwlayer, err := i.layerStores[runtime.GOOS].GetRWLayer(containerID)
	if err != nil {
		logrus.Errorf("Failed to compute size of container rootfs %v: %v", containerID, err)
		return sizeRw, sizeRootfs
	}
	defer i.layerStores[runtime.GOOS].ReleaseRWLayer(rwlayer)

	sizeRw, err = rwlayer.Size()
	if err != nil {
		logrus.Errorf("Driver %s couldn't return diff size of container %s: %s",
			i.layerStores[runtime.GOOS].DriverName(), containerID, err)
		// FIXME: GetSize should return an error. Not changing it now in case
		// there is a side-effect.
		sizeRw = -1
	}

	if parent := rwlayer.Parent(); parent != nil {
		sizeRootfs, err = parent.Size()
		if err != nil {
			sizeRootfs = -1
		} else if sizeRw != -1 {
			sizeRootfs += sizeRw
		}
	}
	return sizeRw, sizeRootfs
}
