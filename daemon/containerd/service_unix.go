//go:build linux || freebsd

package containerd

import (
	"github.com/containerd/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
)

// GetLayerFolders returns the layer folders from an image RootFS.
func (i *ImageService) GetLayerFolders(img *image.Image, rwLayer layer.RWLayer, containerID string) ([]string, error) {
	return nil, errdefs.ErrNotImplemented
}
