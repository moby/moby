//go:build linux || freebsd

package containerd

import (
	"github.com/docker/docker/daemon/container"
	"github.com/docker/docker/daemon/internal/errdefs"
	"github.com/docker/docker/daemon/internal/image"
	"github.com/pkg/errors"
)

// GetLayerFolders returns the layer folders from an image RootFS.
func (i *ImageService) GetLayerFolders(img *image.Image, rwLayer container.RWLayer, containerID string) ([]string, error) {
	return nil, errdefs.NotImplemented(errors.New("not implemented"))
}
