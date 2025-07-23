//go:build linux || freebsd

package containerd

import (
	"github.com/pkg/errors"

	"github.com/docker/docker/daemon/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
)

// GetLayerFolders returns the layer folders from an image RootFS.
func (i *ImageService) GetLayerFolders(img *image.Image, rwLayer container.RWLayer, containerID string) ([]string, error) {
	return nil, errdefs.NotImplemented(errors.New("not implemented"))
}
