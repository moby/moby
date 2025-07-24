//go:build linux || freebsd

package containerd

import (
	"github.com/moby/moby/daemon/container"
	"github.com/moby/moby/daemon/internal/errdefs"
	"github.com/moby/moby/daemon/internal/image"
	"github.com/pkg/errors"
)

// GetLayerFolders returns the layer folders from an image RootFS.
func (i *ImageService) GetLayerFolders(img *image.Image, rwLayer container.RWLayer, containerID string) ([]string, error) {
	return nil, errdefs.NotImplemented(errors.New("not implemented"))
}
